import React from "react";
import axios from "axios";
import PageComponent, { PageComponentProps } from "@engine/containers/PageComponent";
import { Field, MODES } from "@engine/fields";
import { t, subscribe } from "@engine/controllers/i18n";
import ActionTerminal from "./ActionTerminal";

const HDR = { headers: { "X-Request-Type": "api" } };

interface ActionLogEntry {
	time: string;
	result?: string;
	error?: string;
}

interface ActionSnapshot {
	id: string;
	name: string;
	description: string;
	interval_seconds: number;
	running: boolean;
	interactive?: boolean;
	last_run?: string;
	last_result?: string;
	last_error?: string;
	log?: ActionLogEntry[];
	params?: any[];
	last_params?: Record<string, any>;
}

interface State {
	action: ActionSnapshot | null;
	notFound: boolean;
	error: string;
	formData: Record<string, any>;
	running: boolean;
	runKey: number;
	intervalDraft: string;
}

// The per-action detail view: reached by clicking a row in Actions.tsx (see
// Actions.static extraRoutes = [{ href: "/actions/:id", component:
// ActionDetail }] — same mechanism Profile.tsx uses for "/u/:id"). Everything
// that used to be crammed into one Actions.tsx list card (params, schedule,
// terminal/log) lives here instead; the list itself now only shows
// name/description/Run.
class ActionDetail extends PageComponent<PageComponentProps, State> {
	state: State = {
		action: null,
		notFound: false,
		error: "",
		formData: {},
		running: false,
		runKey: 0,
		intervalDraft: "",
	};

	private unsubscribeI18n?: () => void;
	private poll?: ReturnType<typeof setInterval>;

	async componentDidMount() {
		this.unsubscribeI18n = subscribe(() => this.forceUpdate());
		await this.load();
		this.poll = setInterval(this.load, 2000);
	}

	componentWillUnmount() {
		this.unsubscribeI18n?.();
		if (this.poll) clearInterval(this.poll);
	}

	private load = async () => {
		const id = this.props.params?.id;
		if (!id) return;
		try {
			const res = await axios.get("/api/actions", HDR);
			const action: ActionSnapshot | undefined = (res.data?.actions || []).find((a: ActionSnapshot) => a.id === id);
			if (!action) {
				this.setState({ notFound: true });
				return;
			}
			const defaults: Record<string, any> = {};
			for (const p of action.params || []) {
				if (p.default_value !== undefined && p.default_value !== null) defaults[p.name] = p.default_value;
			}
			this.setState((s) => ({
				action,
				notFound: false,
				error: "",
				running: action.running,
				// Each field's own default_value seeds the form first (never shown before
				// this fix - only last_params was); last_params then wins over it for a
				// field an earlier run actually set. The admin's own edits afterward
				// (tracked in formData) are never clobbered by a later poll.
				formData: Object.keys(s.formData).length > 0 ? s.formData : { ...defaults, ...(action.last_params || {}) },
				intervalDraft: s.intervalDraft || String(action.interval_seconds || ""),
			}));
		} catch (e: any) {
			this.setState({ error: e?.message || "request failed" });
		}
	};

	private onChange = (name: string, value: any) => {
		this.setState((s) => ({ formData: { ...s.formData, [name]: value } }));
	};

	private run = async () => {
		const { action, formData } = this.state;
		if (!action) return;
		this.setState({ running: true });
		try {
			await axios.post(`/api/actions/${action.id}/run`, { params: formData }, HDR);
		} catch {
			// The result/error is picked up from the next poll either way.
		}
		this.setState((s) => ({ runKey: s.runKey + 1 }));
		this.load();
	};

	private setSchedule = async () => {
		const { action, intervalDraft } = this.state;
		if (!action) return;
		try {
			await axios.post(`/api/actions/${action.id}/schedule`, { interval_seconds: Number(intervalDraft) || 0 }, HDR);
		} catch {
			// Reflected by the next poll either way.
		}
		this.load();
	};

	render() {
		const { action, notFound, error, formData, running, runKey, intervalDraft } = this.state;

		if (notFound) {
			return (
				<div style={{ padding: 16 }}>
					<div className="alert alert-warning">{t("Unknown action.")}</div>
				</div>
			);
		}
		if (!action) {
			return (
				<div style={{ padding: 16 }}>
					{error ? <div style={{ color: "#f77" }}>{error}</div> : <div className="spinner-border" role="status" />}
				</div>
			);
		}

		return (
			<div style={{ padding: 16, maxWidth: 640 }}>
				<a href="/actions" className="small text-muted d-inline-block mb-2">
					{t("← Actions")}
				</a>
				<h2 style={{ marginBottom: 4 }}>{action.name}</h2>
				<div style={{ color: "#888", marginBottom: 16 }}>{action.description}</div>

				{error && <div style={{ color: "#f77", marginBottom: 12 }}>{error}</div>}

				{action.params && action.params.length > 0 && (
					<div className="mb-3">
						{action.params.map((p) => (
							<div key={p.name} className="mb-2">
								<label className="form-label small text-muted mb-1">{t(p.label || p.name)}</label>
								<Field
									field={p}
									value={formData[p.name]}
									onChange={(v) => this.onChange(p.name, v)}
									mode={MODES.CREATE}
									disabled={running}
									label=""
								/>
								{p.description && <div className="text-muted small">{t(p.description)}</div>}
							</div>
						))}
					</div>
				)}

				<button type="button" className="btn btn-primary btn-sm mb-3" onClick={this.run} disabled={running}>
					{running ? "…" : t("Run now")}
				</button>

				<div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 16 }}>
					<label className="form-label mb-0 small text-muted">{t("Repeat every (seconds, 0 = off)")}</label>
					<input
						type="number"
						min={0}
						className="form-control form-control-sm"
						style={{ width: 110 }}
						value={intervalDraft}
						onChange={(e) => this.setState({ intervalDraft: e.target.value })}
					/>
					<button type="button" className="btn btn-outline-secondary btn-sm" onClick={this.setSchedule}>
						{t("Save")}
					</button>
					{action.interval_seconds > 0 && (
						<span style={{ color: "#888", fontSize: 12 }}>
							{t("Currently every")} {action.interval_seconds}s
						</span>
					)}
				</div>

				{action.interactive && <ActionTerminal key={runKey} id={action.id} onDone={this.load} />}

				{action.log && action.log.length > 0 && (
					<div style={{ marginTop: 10 }}>
						<div className="small text-muted mb-1">{t("Execution log")}</div>
						<div
							style={{
								background: "#111",
								color: "#ddd",
								fontFamily: "monospace",
								fontSize: 12,
								padding: "8px 10px",
								borderRadius: 4,
								maxHeight: 240,
								overflowY: "auto",
							}}
						>
							{action.log.map((entry, i) => (
								<div key={i} style={{ marginBottom: 4 }}>
									<span style={{ color: "#888" }}>{new Date(entry.time).toLocaleString()}</span>{" "}
									{entry.error ? <span style={{ color: "#f77" }}>{entry.error}</span> : <span>{entry.result}</span>}
								</div>
							))}
						</div>
					</div>
				)}
			</div>
		);
	}
}

export default ActionDetail;
