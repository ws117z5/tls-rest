import React from "react";
import axios from "axios";
import PageComponent from "@engine/containers/PageComponent";
import { t, subscribe } from "@engine/i18n";

interface ActionSnapshot {
	id: string;
	name: string;
	description: string;
	interval_seconds: number;
	running: boolean;
	last_run?: string;
	last_result?: string;
	last_error?: string;
}

interface ActionsState {
	actions: ActionSnapshot[];
	error: string;
	runningId: string;
	intervalDrafts: Record<string, string>;
}

// Admin-only page over the actions registry (engine/controllers/actions):
// every admin-triggerable job registered by a package's Init, runnable now
// or on a recurring interval (in-memory only — schedules reset on restart).
class Actions extends PageComponent<{}, ActionsState> {
	constructor(props: {}) {
		super(props);
		this.title = "Actions";
		this.href = "actions";
		this.submenu = "engine";
		this.requiresAuth = true;
		this.requiresAdmin = true;
		this.state = { actions: [], error: "", runningId: "", intervalDrafts: {} };
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

	run = async (id: string) => {
		this.setState({ runningId: id });
		try {
			await axios.post(`/api/actions/${id}/run`, null, { headers: { "X-Request-Type": "api" } });
		} catch {
			// The result/error is picked up from the refreshed snapshot below either way.
		}
		this.setState({ runningId: "" });
		this.load();
	};

	setSchedule = async (id: string, seconds: number) => {
		try {
			await axios.post(`/api/actions/${id}/schedule`, { interval_seconds: seconds }, { headers: { "X-Request-Type": "api" } });
		} catch {
			// Reflected by the refreshed snapshot either way.
		}
		this.load();
	};

	render() {
		const { actions, error, runningId, intervalDrafts } = this.state;
		return (
			<div style={{ padding: 16 }}>
				<h2 style={{ marginBottom: 12 }}>{t("Actions")}</h2>

				{error && <div style={{ color: "#f77", marginBottom: 12 }}>{error}</div>}

				{actions.length === 0 && !error && <div style={{ color: "#888" }}>{t("No actions registered.")}</div>}

				<div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
					{actions.map((a) => {
						const draft = intervalDrafts[a.id] ?? String(a.interval_seconds || "");
						return (
							<div key={a.id} className="card card-body py-2">
								<div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 12, flexWrap: "wrap" }}>
									<div>
										<h4 style={{ margin: 0 }}>{a.name}</h4>
										<div style={{ color: "#888", fontSize: 13 }}>{a.description}</div>
									</div>
									<button
										type="button"
										className="btn btn-primary btn-sm"
										onClick={() => this.run(a.id)}
										disabled={a.running || runningId === a.id}
									>
										{a.running || runningId === a.id ? "…" : t("Run now")}
									</button>
								</div>

								<div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 10 }}>
									<label className="form-label mb-0 small text-muted">{t("Repeat every (seconds, 0 = off)")}</label>
									<input
										type="number"
										min={0}
										className="form-control form-control-sm"
										style={{ width: 110 }}
										value={draft}
										onChange={(e) => this.setState({ intervalDrafts: { ...intervalDrafts, [a.id]: e.target.value } })}
									/>
									<button
										type="button"
										className="btn btn-outline-secondary btn-sm"
										onClick={() => this.setSchedule(a.id, Number(draft) || 0)}
									>
										{t("Save")}
									</button>
									{a.interval_seconds > 0 && (
										<span style={{ color: "#888", fontSize: 12 }}>
											{t("Currently every")} {a.interval_seconds}s
										</span>
									)}
								</div>

								{(a.last_run || a.last_result || a.last_error) && (
									<div style={{ marginTop: 10, fontSize: 12, color: "#888" }}>
										{a.last_run && (
											<div>
												{t("Last run")}: {new Date(a.last_run).toLocaleString()}
											</div>
										)}
										{a.last_result && <div>{a.last_result}</div>}
										{a.last_error && <div style={{ color: "#f77" }}>{a.last_error}</div>}
									</div>
								)}
							</div>
						);
					})}
				</div>
			</div>
		);
	}
}

export default Actions;
