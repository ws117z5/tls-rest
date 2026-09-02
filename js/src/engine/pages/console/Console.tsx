import React from "react";
import axios from "axios";
import PageComponent from "@engine/containers/PageComponent";
import Config from "@engine/Config";

interface ConsoleState {
	command: string;
	history: Array<{ cmd: string; out: string; err?: string }>;
	busy: boolean;
}

// Admin-only live actions console. Sends a command line to POST /api/console
// (backed by the input controller) and shows the output. Admin gating is
// enforced both here (requiresAdmin) and server-side.
class Console extends PageComponent<{}, ConsoleState> {
	constructor(props: {}) {
		super(props);
		this.title = "Console";
		this.href = "console";
		this.submenu = "engine";
		this.requiresAuth = true;
		this.requiresAdmin = true;
		this.state = { command: "", history: [], busy: false };
	}

	run = async () => {
		const cmd = this.state.command.trim();
		if (!cmd || this.state.busy) return;
		this.setState({ busy: true });
		try {
			const res = await axios.post(
				Config.serverURL + "api/console",
				{ command: cmd },
				{ headers: { "X-Request-Type": "api" } }
			);
			const { output = "", error } = res.data || {};
			this.setState((s) => ({
				command: "",
				busy: false,
				history: [...s.history, { cmd, out: output, err: error }],
			}));
		} catch (e: any) {
			this.setState((s) => ({
				busy: false,
				history: [...s.history, { cmd, out: "", err: e?.message || "request failed" }],
			}));
		}
	};

	onKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
		if (e.key === "Enter") this.run();
	};

	render() {
		const mono: React.CSSProperties = {
			fontFamily: "ui-monospace, Menlo, Consolas, monospace",
			fontSize: 13,
		};
		return (
			<div style={{ padding: 16, ...mono }}>
				<h2 style={{ marginBottom: 8 }}>Console</h2>
				<div style={{ color: "#888", marginBottom: 12 }}>
					Admin actions console. Type <code>help</code> for commands.
				</div>

				<div
					style={{
						background: "#111",
						color: "#e6e6e6",
						padding: 12,
						borderRadius: 6,
						minHeight: 240,
						maxHeight: "50vh",
						overflow: "auto",
						whiteSpace: "pre-wrap",
						...mono,
					}}
				>
					{this.state.history.length === 0 && (
						<div style={{ color: "#666" }}>No output yet.</div>
					)}
					{this.state.history.map((h, i) => (
						<div key={i} style={{ marginBottom: 10 }}>
							<div style={{ color: "#7fd" }}>&gt; {h.cmd}</div>
							{h.out && <div>{h.out}</div>}
							{h.err && <div style={{ color: "#f77" }}>error: {h.err}</div>}
						</div>
					))}
				</div>

				<div style={{ display: "flex", gap: 8, marginTop: 10 }}>
					<input
						value={this.state.command}
						onChange={(e) => this.setState({ command: e.target.value })}
						onKeyDown={this.onKey}
						placeholder="e.g. db query select id, email from users limit 5"
						style={{ flex: 1, padding: "8px 10px", ...mono }}
						autoFocus
					/>
					<button onClick={this.run} disabled={this.state.busy} style={{ padding: "8px 16px" }}>
						{this.state.busy ? "…" : "Run"}
					</button>
				</div>
			</div>
		);
	}
}

export default Console;