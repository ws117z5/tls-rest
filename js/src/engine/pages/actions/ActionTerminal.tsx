import React, { useCallback, useEffect, useRef, useState } from "react";
import axios from "axios";
import { t } from "@engine/controllers/i18n";

const HDR = { headers: { "X-Request-Type": "api" } };
const ANSI = /\x1b\[[0-9;?]*[ -/]*[@-~]/g;

// Live terminal for a Command action: polls its output, and types into it (Enter, y/n, password) on request.
const ActionTerminal: React.FC<{ id: string; onDone: () => void }> = ({ id, onDone }) => {
	const [text, setText] = useState("");
	const [running, setRunning] = useState(false);
	const [input, setInput] = useState("");
	const next = useRef(0);
	const wasRunning = useRef(false);
	const endRef = useRef<HTMLDivElement>(null);

	const poll = useCallback(async () => {
		try {
			const res = await axios.get(`/api/actions/${id}/output`, { params: { from: next.current }, ...HDR });
			const d = res.data || {};
			if (d.text) setText((s) => s + d.text);
			next.current = d.next ?? next.current;
			if (wasRunning.current && !d.running) onDone();
			wasRunning.current = !!d.running;
			setRunning(!!d.running);
		} catch {
			// next tick retries
		}
	}, [id, onDone]);

	useEffect(() => {
		poll();
		const timer = setInterval(poll, 800);
		return () => clearInterval(timer);
	}, [poll]);

	useEffect(() => {
		endRef.current?.scrollIntoView({ block: "nearest" });
	}, [text]);

	const clean = text.replace(ANSI, "").replace(/\r\n/g, "\n").replace(/\r/g, "");
	const lastLine = clean.split("\n").pop() || "";
	const asksPassword = running && /password[^\n]*:\s*$/i.test(lastLine);

	const send = async (value: string, enter: boolean) => {
		try {
			await axios.post(`/api/actions/${id}/input`, { text: value, enter }, HDR);
		} catch {
			// not running anymore
		}
		setInput("");
		poll();
	};

	const stop = async () => {
		try {
			await axios.post(`/api/actions/${id}/stop`, null, HDR);
		} catch {
			// already stopped
		}
	};

	return (
		<div style={{ marginTop: 10 }}>
			<div className="small text-muted mb-1">{t("Terminal")}</div>
			<pre
				style={{
					background: "#111",
					color: "#ddd",
					fontSize: 12,
					padding: "8px 10px",
					borderRadius: 4,
					minHeight: 60,
					maxHeight: 360,
					overflowY: "auto",
					whiteSpace: "pre-wrap",
					margin: 0,
				}}
			>
				{clean}
				<div ref={endRef} />
			</pre>
			{running && (
				<form
					className="d-flex align-items-center gap-2 mt-2"
					onSubmit={(e) => {
						e.preventDefault();
						send(input, true);
					}}
				>
					<input
						type={asksPassword ? "password" : "text"}
						autoFocus={asksPassword}
						className="form-control form-control-sm"
						placeholder={asksPassword ? t("Password requested — type it here") : t("Type a reply, Enter to send")}
						value={input}
						onChange={(e) => setInput(e.target.value)}
					/>
					<button type="submit" className="btn btn-primary btn-sm">{t("Send")}</button>
					<button type="button" className="btn btn-outline-secondary btn-sm" onClick={() => send("y", false)}>y</button>
					<button type="button" className="btn btn-outline-secondary btn-sm" onClick={() => send("n", false)}>n</button>
					<button type="button" className="btn btn-outline-danger btn-sm" onClick={stop}>{t("Stop")}</button>
				</form>
			)}
		</div>
	);
};

export default ActionTerminal;
