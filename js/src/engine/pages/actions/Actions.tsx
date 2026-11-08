import React from "react";
import axios from "axios";
import PageComponent, { PageComponentProps } from "@engine/containers/PageComponent";
import { t, subscribe } from "@engine/controllers/i18n";
import ActionDetail from "./ActionDetail";

interface ActionSnapshot {
	id: string;
	name: string;
	description: string;
	running: boolean;
	interactive?: boolean;
	params?: any[];
}

interface ActionsState {
	actions: ActionSnapshot[];
	error: string;
	runningId: string;
}

// Admin-only page over the actions registry (engine/controllers/actions):
// every admin-triggerable job registered by a package's Init, runnable now
// or on a recurring interval (in-memory only — schedules reset on restart).
// Just the list here (name/description/Run); clicking a row — or Run, for an
// action that takes params — opens its own detail view (ActionDetail, via
// extraRoutes below) with the full params form, schedule control, and log.
class Actions extends PageComponent<PageComponentProps, ActionsState> {
	// Reached via "/actions/:id" — same mechanism Profile.tsx uses for "/u/:id".
	static extraRoutes = [{ href: "/actions/:id", component: ActionDetail }];

	constructor(props: PageComponentProps) {
		super(props);
		this.title = "Actions";
		this.href = "actions";
		this.submenu = "engine";
		this.requiresAuth = true;
		this.requiresAdmin = true;
		this.state = { actions: [], error: "", runningId: "" };
	}

	private unsubscribeI18n?: () => void;
	async componentDidMount() {
		await super.componentDidMount();
		this.unsubscribeI18n = subscribe(() => this.forceUpdate());
		this.load();
	}
	componentWillUnmount() {
		this.unsubscribeI18n?.();
	}

	load = async () => {
		try {
			const res = await axios.get("/api/actions", { headers: { "X-Request-Type": "api" } });
			this.setState({ actions: res.data?.actions || [], error: "" });
		} catch (e: any) {
			this.setState({ error: e?.message || "request failed" });
		}
	};

	// Only a parameterless action runs straight from the list — one with
	// Params has nothing to submit yet, so Run opens its detail view instead
	// (same as clicking the row itself).
	run = async (a: ActionSnapshot) => {
		if (a.params && a.params.length > 0) {
			this.props.navigate(`/actions/${a.id}`);
			return;
		}
		this.setState({ runningId: a.id });
		try {
			await axios.post(`/api/actions/${a.id}/run`, null, { headers: { "X-Request-Type": "api" } });
		} catch {
			// The result/error is picked up from the refreshed snapshot below either way.
		}
		this.setState({ runningId: "" });
		this.load();
	};

	render() {
		const { actions, error, runningId } = this.state;
		return (
			<div style={{ padding: 16 }}>
				<h2 style={{ marginBottom: 12 }}>{t("Actions")}</h2>

				{error && <div style={{ color: "#f77", marginBottom: 12 }}>{error}</div>}

				{actions.length === 0 && !error && <div style={{ color: "#888" }}>{t("No actions registered.")}</div>}

				<div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
					{actions.map((a) => (
						<div
							key={a.id}
							className="card card-body py-2"
							style={{ cursor: "pointer" }}
							onClick={() => this.props.navigate(`/actions/${a.id}`)}
						>
							<div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 12, flexWrap: "wrap" }}>
								<div>
									<h4 style={{ margin: 0 }}>{a.name}</h4>
									<div style={{ color: "#888", fontSize: 13 }}>{a.description}</div>
								</div>
								<button
									type="button"
									className="btn btn-primary btn-sm"
									onClick={(e) => {
										e.stopPropagation();
										this.run(a);
									}}
									disabled={a.running || runningId === a.id}
								>
									{a.running || runningId === a.id ? "…" : t("Run now")}
								</button>
							</div>
						</div>
					))}
				</div>
			</div>
		);
	}
}

export default Actions;
