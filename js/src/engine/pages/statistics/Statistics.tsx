import React, { useState } from "react";
import axios from "axios";
import PageComponent from "@engine/containers/PageComponent";
import Config from "@engine/Config";
import CustomSelect from "@engine/fields/CustomSelect";
import { t, subscribe } from "@engine/i18n";

interface BreakdownRow {
	value: string;
	count: number;
}

interface StatsResponse {
	total_requests: number;
	blocked_requests: number;
	unique_sessions: number;
	new_users: number;
	by_module: BreakdownRow[];
	by_path: BreakdownRow[];
	by_country: BreakdownRow[];
	by_ip: BreakdownRow[];
	by_user_agent: BreakdownRow[];
}

// LiveSnapshot is one raw poll of /api/statistics/live: cumulative counters
// (except the *_bytes gauges) — rates are derived client-side by diffing
// consecutive snapshots, since the server keeps no history of its own.
interface LiveSnapshot {
	time: string;
	requests_total: number;
	db_queries_total: number;
	cpu_seconds_total: number;
	memory_bytes: number;
	app_heap_bytes: number;
	engine_overhead_bytes: number;
	db_size_bytes: number;
}

type BreakdownKey = "module" | "path" | "country" | "ip" | "user_agent";

const maxLiveSnapshots = 60;

type Period = "day" | "week" | "month" | "year" | "ever";
type BreakdownView = "table" | "bar" | "pie";

const PERIOD_OPTIONS: { value: Period; label: string }[] = [
	{ value: "day", label: "Past day" },
	{ value: "week", label: "Past week" },
	{ value: "month", label: "Past month" },
	{ value: "year", label: "Past year" },
	{ value: "ever", label: "Ever" },
];

const VIEW_OPTIONS: { value: BreakdownView; label: string }[] = [
	{ value: "table", label: "Table" },
	{ value: "bar", label: "Bar" },
	{ value: "pie", label: "Pie" },
];

// periodRange turns a preset into {from,to} "YYYY-MM-DD" bounds for the
// backend; "ever" means both are omitted (unbounded).
function periodRange(period: Period): { from: string; to: string } {
	if (period === "ever") return { from: "", to: "" };
	const to = new Date();
	const from = new Date(to);
	if (period === "day") from.setDate(to.getDate() - 1);
	else if (period === "week") from.setDate(to.getDate() - 7);
	else if (period === "month") from.setMonth(to.getMonth() - 1);
	else if (period === "year") from.setFullYear(to.getFullYear() - 1);
	return { from: from.toISOString().slice(0, 10), to: to.toISOString().slice(0, 10) };
}

interface StatisticsState {
	moduleFilter: string;
	pathFilter: string;
	countryFilter: string;
	ipFilter: string;
	userAgentFilter: string;
	period: Period;
	views: Record<BreakdownKey, BreakdownView>;
	loading: boolean;
	error: string;
	data: StatsResponse | null;
	liveSnapshots: LiveSnapshot[];
	refreshPresets: number[];
	refreshRate: number;
	geoipLoaded: boolean;
	geoipNote: string;
}

interface LiveSeries {
	key: string;
	label: string;
	color: string;
	values: number[];
	unit: string;
}

// LiveChart overlays every series on one full-width SVG, each independently
// normalized to the same 0-1 band (their units/scales differ too much to
// share an axis) — a shared legend shows current values, and hovering shows
// each series' REAL value at that point via a tracked mouse-x index.
const LiveChart: React.FC<{ series: LiveSeries[]; times: string[] }> = ({ series, times }) => {
	const [hover, setHover] = useState<number | null>(null);
	const n = Math.max(0, ...series.map((s) => s.values.length));
	const width = 1000;
	const height = 260;
	const padLeft = 44;
	const padRight = 10;
	const padTop = 10;
	const padBottom = 24;
	const plotW = width - padLeft - padRight;
	const plotH = height - padTop - padBottom;

	const legend = (
		<div style={{ display: "flex", flexWrap: "wrap", gap: 16, marginBottom: 8 }}>
			{series.map((s) => {
				const latest = s.values.length ? s.values[s.values.length - 1] : 0;
				return (
					<div key={s.key} style={{ display: "flex", alignItems: "center", gap: 6 }}>
						<span style={{ width: 12, height: 12, background: s.color, display: "inline-block", borderRadius: 2 }} />
						<span>
							{t(s.label)}: {latest.toFixed(1)} {s.unit}
						</span>
					</div>
				);
			})}
		</div>
	);

	if (n < 2) {
		return (
			<div>
				{legend}
				<div style={{ color: "#888" }}>{t("No data for this range.")}</div>
			</div>
		);
	}

	const stepX = plotW / (n - 1);
	const pointsFor = (values: number[]) => {
		const max = Math.max(...values, 0.0001);
		const min = Math.min(...values, 0);
		const span = max - min || 1;
		return values.map((v, i) => `${padLeft + i * stepX},${padTop + (1 - (v - min) / span) * plotH}`).join(" ");
	};

	// Y is each series' own 0-1 range (see the module doc above), so the axis
	// reads relative position, not a shared unit — hover always shows the
	// real value instead.
	const yTicks = [0, 0.25, 0.5, 0.75, 1];
	const xTickCount = Math.min(6, n);
	const xTickIndices = Array.from({ length: xTickCount }, (_, i) => Math.round((i * (n - 1)) / (xTickCount - 1 || 1)));

	const onMove = (e: React.MouseEvent<SVGRectElement>) => {
		const rect = e.currentTarget.getBoundingClientRect();
		const ratio = (e.clientX - rect.left) / rect.width;
		setHover(Math.min(Math.max(Math.round(ratio * (n - 1)), 0), n - 1));
	};

	const hoverX = hover !== null ? padLeft + hover * stepX : 0;

	return (
		<div>
			{legend}
			<div style={{ position: "relative", width: "100%" }}>
				<svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none">
					<rect x={0} y={0} width={width} height={height} fill="#fff" />
					{yTicks.map((frac) => {
						const y = padTop + (1 - frac) * plotH;
						return (
							<g key={frac}>
								<line x1={padLeft} x2={width - padRight} y1={y} y2={y} stroke="#eee" strokeWidth={1} />
								<text x={padLeft - 6} y={y + 3} textAnchor="end" fontSize={9} fontFamily="Arial, sans-serif" fill="#999">
									{Math.round(frac * 100)}%
								</text>
							</g>
						);
					})}
					{xTickIndices.map((idx) => {
						const x = padLeft + idx * stepX;
						return (
							<g key={idx}>
								<line x1={x} x2={x} y1={padTop} y2={height - padBottom} stroke="#f3f3f3" strokeWidth={1} />
								<text x={x} y={height - padBottom + 14} textAnchor="middle" fontSize={9} fontFamily="Arial, sans-serif" fill="#999">
									{times[idx]}
								</text>
							</g>
						);
					})}
					<rect x={padLeft} y={padTop} width={plotW} height={plotH} fill="none" stroke="#ddd" strokeWidth={1} />
					{series.map((s) => (
						<polyline key={s.key} points={pointsFor(s.values)} fill="none" stroke={s.color} strokeWidth={2} />
					))}
					{hover !== null && (
						<line x1={hoverX} x2={hoverX} y1={padTop} y2={height - padBottom} stroke="#888" strokeWidth={1} strokeDasharray="4 3" />
					)}
					<rect
						x={padLeft}
						y={padTop}
						width={plotW}
						height={plotH}
						fill="transparent"
						onMouseMove={onMove}
						onMouseLeave={() => setHover(null)}
					/>
				</svg>
				{hover !== null && (
					<div
						style={{
							position: "absolute",
							top: 4,
							left: `${(hoverX / width) * 100}%`,
							transform: hoverX / width > 0.6 ? "translateX(-100%)" : undefined,
							background: "#222",
							color: "#eee",
							padding: "6px 10px",
							borderRadius: 4,
							fontSize: 12,
							pointerEvents: "none",
							whiteSpace: "nowrap",
							zIndex: 1,
						}}
					>
						<div style={{ color: "#999", marginBottom: 4 }}>{times[hover]}</div>
						{series.map((s) => (
							<div key={s.key}>
								<span style={{ color: s.color }}>●</span> {t(s.label)}: {(s.values[hover] ?? 0).toFixed(1)} {s.unit}
							</div>
						))}
					</div>
				)}
			</div>
		</div>
	);
};

const PIE_COLORS = ["#2bb", "#c93", "#d44", "#55c", "#9c6", "#c69", "#6cc", "#cc6", "#69c", "#c96"];

function ellipsize(s: string, max: number): string {
	return s.length > max ? s.slice(0, max - 1) + "…" : s;
}

// formatBytes picks MB or GB depending on magnitude, for the memory breakdown.
function formatBytes(bytes: number): string {
	const gb = bytes / (1024 * 1024 * 1024);
	if (gb >= 1) return `${gb.toFixed(2)} GB`;
	return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// BarChart renders each row as a horizontal bar scaled to the row's share of
// the largest count in the set.
function BarChart({ rows }: { rows: BreakdownRow[] }) {
	const max = Math.max(...rows.map((r) => r.count), 1);
	return (
		<div>
			{rows.map((r) => (
				<div key={r.value} style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 4 }}>
					<div style={{ width: 110, fontSize: 12, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={r.value}>
						{ellipsize(r.value, 22)}
					</div>
					<div style={{ flex: 1, background: "#e5e5e5", borderRadius: 2 }}>
						<div style={{ width: `${(r.count / max) * 100}%`, background: "#2bb", height: 14, borderRadius: 2 }} />
					</div>
					<div style={{ width: 36, fontSize: 12, textAlign: "right" }}>{r.count}</div>
				</div>
			))}
		</div>
	);
}

// PieChart renders each row as a slice sized to its share of the set's total,
// with a swatch legend alongside (native <title> gives a hover tooltip).
function PieChart({ rows }: { rows: BreakdownRow[] }) {
	const total = rows.reduce((sum, r) => sum + r.count, 0) || 1;
	const size = 150;
	const r = size / 2;
	let angle = -90;
	const slices = rows.map((row, i) => {
		const start = angle;
		const end = angle + (row.count / total) * 360;
		angle = end;
		const large = end - start > 180 ? 1 : 0;
		const toXY = (deg: number) => {
			const rad = (deg * Math.PI) / 180;
			return [r + r * Math.cos(rad), r + r * Math.sin(rad)];
		};
		const [x1, y1] = toXY(start);
		const [x2, y2] = toXY(end);
		return (
			<path key={row.value} d={`M ${r},${r} L ${x1},${y1} A ${r},${r} 0 ${large} 1 ${x2},${y2} Z`} fill={PIE_COLORS[i % PIE_COLORS.length]}>
				<title>
					{row.value}: {row.count}
				</title>
			</path>
		);
	});
	return (
		<div style={{ display: "flex", alignItems: "center", gap: 12 }}>
			<svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
				{slices}
			</svg>
			<div style={{ fontSize: 12 }}>
				{rows.map((row, i) => (
					<div key={row.value} style={{ display: "flex", alignItems: "center", gap: 4, marginBottom: 2 }}>
						<span style={{ width: 10, height: 10, background: PIE_COLORS[i % PIE_COLORS.length], display: "inline-block", flexShrink: 0 }} />
						<span title={row.value}>{ellipsize(row.value, 24)}</span>
						<span style={{ color: "#888" }}>({row.count})</span>
					</div>
				))}
			</div>
		</div>
	);
}

// Admin-only analytics page: access_log counts/breakdowns (module/page/
// country/ip/user-agent) filtered by any of those plus a time-range preset,
// plus a live graph of requests/db-queries/CPU/memory. CPU/memory are
// process-wide and can't be scoped the way the requests rate can.
class Statistics extends PageComponent<{}, StatisticsState> {
	constructor(props: {}) {
		super(props);
		this.title = "Statistics";
		this.href = "statistics";
		this.submenu = "engine";
		this.requiresAuth = true;
		this.requiresAdmin = true;
		this.state = {
			moduleFilter: "",
			pathFilter: "",
			countryFilter: "",
			ipFilter: "",
			userAgentFilter: "",
			period: "week",
			views: { module: "table", path: "table", country: "table", ip: "table", user_agent: "table" },
			loading: false,
			error: "",
			data: null,
			liveSnapshots: [],
			refreshPresets: [],
			refreshRate: 0,
			geoipLoaded: false,
			geoipNote: "",
		};
	}

	private unsubscribeI18n?: () => void;
	private liveTimer?: ReturnType<typeof setInterval>;

	async componentDidMount() {
		await super.componentDidMount();
		this.unsubscribeI18n = subscribe(() => this.forceUpdate());
		this.loadGeoIPStatus();
		this.load();
		this.loadLive();
	}
	componentWillUnmount() {
		this.unsubscribeI18n?.();
		if (this.liveTimer) clearInterval(this.liveTimer);
	}

	// currentParams mirrors the active filters as /api/statistics query params —
	// shared by the stats fetch and the geoip backfill so both scope to the same rows.
	private currentParams(): Record<string, string> {
		const { moduleFilter, pathFilter, countryFilter, ipFilter, userAgentFilter, period } = this.state;
		const { from, to } = periodRange(period);
		const params: Record<string, string> = {};
		if (moduleFilter) params.module = moduleFilter;
		if (pathFilter) params.path = pathFilter;
		if (countryFilter) params.country = countryFilter;
		if (ipFilter) params.ip = ipFilter;
		if (userAgentFilter) params.user_agent = userAgentFilter;
		if (from) params.from = from;
		if (to) params.to = to;
		return params;
	}

	load = async () => {
		const params = this.currentParams();
		this.setState({ loading: true, error: "" });

		// Resolve country for any rows in scope that don't have one yet, using
		// whatever GeoIP tables are currently loaded (a no-op if none are), so
		// the breakdown fetched right after reflects the freshly-resolved rows.
		try {
			const bf = await axios.post("/api/statistics/geoip/backfill", null, {
				params,
				headers: { "X-Request-Type": "api" },
			});
			this.setState({ geoipNote: bf.data?.resolved > 0 ? `${t("Resolved")} ${bf.data.resolved} ${t("countries")}` : "" });
		} catch {
			// Backfill is best-effort; a failure here shouldn't block loading stats.
		}

		try {
			const res = await axios.get("/api/statistics", {
				params,
				headers: { "X-Request-Type": "api" },
			});
			this.setState({ loading: false, data: res.data });
		} catch (e: any) {
			this.setState({ loading: false, error: e?.message || "request failed" });
		}
		// The live panel's "requests" series is module-scoped too, and a switch
		// invalidates any snapshots already collected under the old scope.
		this.setState({ liveSnapshots: [] });
		this.loadLive();
	};

	loadGeoIPStatus = async () => {
		try {
			const res = await axios.get("/api/statistics/geoip/status", { headers: { "X-Request-Type": "api" } });
			this.setState({ geoipLoaded: !!res.data?.loaded });
		} catch {
			// Status is a nice-to-have; leave the last known value.
		}
	};

	loadLive = async () => {
		try {
			const res = await axios.get("/api/statistics/live", {
				params: this.state.moduleFilter ? { module: this.state.moduleFilter } : {},
				headers: { "X-Request-Type": "api" },
			});
			const presets: number[] = res.data?.refresh_presets || [];
			const rate = this.state.refreshRate || presets[0] || 0;
			const snapshot: LiveSnapshot = {
				time: res.data.time,
				requests_total: res.data.requests_total,
				db_queries_total: res.data.db_queries_total,
				cpu_seconds_total: res.data.cpu_seconds_total,
				memory_bytes: res.data.memory_bytes,
				app_heap_bytes: res.data.app_heap_bytes,
				engine_overhead_bytes: res.data.engine_overhead_bytes,
				db_size_bytes: res.data.db_size_bytes,
			};
			this.setState((s) => ({
				liveSnapshots: [...s.liveSnapshots, snapshot].slice(-maxLiveSnapshots),
				refreshPresets: presets,
				refreshRate: rate,
			}));
			this.restartLiveTimer(rate);
		} catch {
			// Live panel is a nice-to-have; a failed poll just leaves the last data up.
		}
	};

	// rates derives per-second (and memory in MB) series from consecutive raw
	// snapshots — the server hands back cumulative counters, not a history.
	private rates() {
		const snaps = this.state.liveSnapshots;
		const requests: number[] = [];
		const dbQueries: number[] = [];
		const cpu: number[] = [];
		const memory: number[] = [];
		for (let i = 0; i < snaps.length; i++) {
			memory.push(snaps[i].memory_bytes / (1024 * 1024));
			if (i === 0) {
				requests.push(0);
				dbQueries.push(0);
				cpu.push(0);
				continue;
			}
			const dt = (new Date(snaps[i].time).getTime() - new Date(snaps[i - 1].time).getTime()) / 1000;
			if (dt <= 0) {
				requests.push(0);
				dbQueries.push(0);
				cpu.push(0);
				continue;
			}
			requests.push((snaps[i].requests_total - snaps[i - 1].requests_total) / dt);
			dbQueries.push((snaps[i].db_queries_total - snaps[i - 1].db_queries_total) / dt);
			cpu.push(((snaps[i].cpu_seconds_total - snaps[i - 1].cpu_seconds_total) / dt) * 100);
		}
		return { requests, dbQueries, cpu, memory };
	}

	private restartLiveTimer(rate: number) {
		if (this.liveTimer) clearInterval(this.liveTimer);
		if (!rate) return;
		this.liveTimer = setInterval(this.loadLive, rate * 1000);
	}

	private onRefreshRateChange = (rate: number) => {
		this.setState({ refreshRate: rate });
		this.restartLiveTimer(rate);
	};

	private setBreakdownView(key: BreakdownKey, view: BreakdownView) {
		this.setState((s) => ({ views: { ...s.views, [key]: view } }));
	}

	private renderBreakdown(key: BreakdownKey, title: string, columnLabel: string, rows: BreakdownRow[] | undefined) {
		const view = this.state.views[key];
		return (
			<div className="card card-body py-2" style={{ minWidth: 260, flex: 1 }}>
				<div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
					<h4 style={{ margin: 0 }}>{t(title)}</h4>
					<CustomSelect
						value={view}
						onChange={(v) => this.setBreakdownView(key, v as BreakdownView)}
						options={VIEW_OPTIONS.map((o) => ({ value: o.value, label: t(o.label) }))}
					/>
				</div>
				{!rows || rows.length === 0 ? (
					<div style={{ color: "#888" }}>{t("No data for this range.")}</div>
				) : view === "bar" ? (
					<BarChart rows={rows} />
				) : view === "pie" ? (
					<PieChart rows={rows} />
				) : (
					<table style={{ width: "100%", borderCollapse: "collapse" }}>
						<thead>
							<tr>
								<th style={{ textAlign: "left", borderBottom: "1px solid #444" }}>{t(columnLabel)}</th>
								<th style={{ textAlign: "right", borderBottom: "1px solid #444" }}>{t("Count")}</th>
							</tr>
						</thead>
						<tbody>
							{rows.map((r) => (
								<tr key={r.value}>
									<td>{r.value}</td>
									<td style={{ textAlign: "right" }}>{r.count}</td>
								</tr>
							))}
						</tbody>
					</table>
				)}
			</div>
		);
	}

	render() {
		const {
			moduleFilter,
			pathFilter,
			countryFilter,
			ipFilter,
			userAgentFilter,
			period,
			loading,
			error,
			data,
			refreshPresets,
			refreshRate,
		} = this.state;
		const modules = Config.getModules();
		const live = this.rates();
		const liveTimes = this.state.liveSnapshots.map((s) => new Date(s.time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }));
		const liveSeries: LiveSeries[] = [
			{ key: "requests", label: moduleFilter ? "Requests (module)" : "Requests", color: "#2bb", values: live.requests, unit: "/s" },
			{ key: "dbQueries", label: "DB queries", color: "#c93", values: live.dbQueries, unit: "/s" },
			{ key: "cpu", label: "CPU load", color: "#d44", values: live.cpu, unit: "%" },
			{ key: "memory", label: "Memory", color: "#55c", values: live.memory, unit: "MB" },
		];

		return (
			<div style={{ padding: 16 }}>
				<h2 style={{ marginBottom: 12 }}>{t("Statistics")}</h2>

				<div className="fieldset-filters card card-body mb-3 py-2">
					<div className="row align-items-end">
						<div className="col-auto">
							<label className="form-label mb-1 small text-muted">{t("Module")}</label>
							<CustomSelect
								value={moduleFilter}
								onChange={(v) => this.setState({ moduleFilter: v })}
								options={[{ value: "", label: t("All") }, ...modules.map((m) => ({ value: m.key, label: m.title }))]}
							/>
						</div>
						<div className="col-auto">
							<label className="form-label mb-1 small text-muted">{t("Page")}</label>
							<input
								type="text"
								className="form-control form-control-sm"
								placeholder="/posts"
								value={pathFilter}
								onChange={(e) => this.setState({ pathFilter: e.target.value })}
							/>
						</div>
						<div className="col-auto">
							<label className="form-label mb-1 small text-muted">{t("Country")}</label>
							<input
								type="text"
								className="form-control form-control-sm"
								placeholder={t("All")}
								maxLength={2}
								value={countryFilter}
								onChange={(e) => this.setState({ countryFilter: e.target.value.toUpperCase() })}
							/>
						</div>
						<div className="col-auto">
							<label className="form-label mb-1 small text-muted">{t("IP")}</label>
							<input
								type="text"
								className="form-control form-control-sm"
								placeholder={t("All")}
								value={ipFilter}
								onChange={(e) => this.setState({ ipFilter: e.target.value })}
							/>
						</div>
						<div className="col-auto">
							<label className="form-label mb-1 small text-muted">{t("User agent")}</label>
							<input
								type="text"
								className="form-control form-control-sm"
								placeholder={t("All")}
								value={userAgentFilter}
								onChange={(e) => this.setState({ userAgentFilter: e.target.value })}
							/>
						</div>
						<div className="col-auto">
							<label className="form-label mb-1 small text-muted">{t("Period")}</label>
							<CustomSelect
								value={period}
								onChange={(v) => this.setState({ period: v as Period })}
								options={PERIOD_OPTIONS.map((p) => ({ value: p.value, label: t(p.label) }))}
							/>
						</div>
						<div className="col-auto">
							<button type="button" className="btn btn-primary btn-sm" onClick={this.load} disabled={loading}>
								{loading ? "…" : t("Apply")}
							</button>
						</div>
						<div className="col-auto">
							<label className="form-label mb-1 small text-muted d-block">
								{this.state.geoipLoaded ? t("GeoIP loaded") : t("GeoIP not loaded — run it from Actions")}
							</label>
						</div>
					</div>
					{this.state.geoipNote && <div style={{ color: "#888", fontSize: 12, marginTop: 6 }}>{this.state.geoipNote}</div>}
				</div>

				{error && <div style={{ color: "#f77", marginBottom: 12 }}>{error}</div>}

				{data && (
					<>
						<div className="card card-body py-2 mb-3">
							<div style={{ display: "flex", gap: 24, flexWrap: "wrap" }}>
								<div>
									<div style={{ fontSize: 24 }}>{data.total_requests}</div>
									<div style={{ color: "#888" }}>{t("Total requests")}</div>
								</div>
								<div>
									<div style={{ fontSize: 24 }}>{data.blocked_requests}</div>
									<div style={{ color: "#888" }}>{t("Blocked requests")}</div>
								</div>
								<div>
									<div style={{ fontSize: 24 }}>{data.unique_sessions}</div>
									<div style={{ color: "#888" }}>{t("Unique sessions")}</div>
								</div>
								<div>
									<div style={{ fontSize: 24 }}>{data.new_users}</div>
									<div style={{ color: "#888" }}>{t("New users")}</div>
									<div style={{ color: "#666", fontSize: 11 }}>
										{t("Only the time period applies to this count — signups aren't tied to a module, page, country, IP, or user agent.")}
									</div>
								</div>
							</div>
						</div>

						<div className="card card-body py-2 mb-3">
							<div style={{ display: "flex", flexWrap: "wrap", gap: 16 }}>
								{this.renderBreakdown("module", "By module", "Module", data.by_module)}
								{this.renderBreakdown("path", "By page", "Page", data.by_path)}
								{this.renderBreakdown("country", "By country", "Country", data.by_country)}
								{this.renderBreakdown("ip", "By IP", "IP", data.by_ip)}
								{this.renderBreakdown("user_agent", "By user agent", "User agent", data.by_user_agent)}
							</div>
						</div>
					</>
				)}

				<div className="card card-body py-2 mb-3">
					<div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 10 }}>
						<h3 style={{ margin: 0 }}>{t("Live")}</h3>
						{refreshPresets.length > 0 && (
							<select value={refreshRate} onChange={(e) => this.onRefreshRateChange(Number(e.target.value))}>
								{refreshPresets.map((p) => (
									<option key={p} value={p}>
										{p}s
									</option>
								))}
							</select>
						)}
					</div>
					<LiveChart series={liveSeries} times={liveTimes} />

					{this.state.liveSnapshots.length > 0 && (
						<div style={{ marginTop: 16 }}>
							<h4 style={{ marginBottom: 6 }}>{t("Memory breakdown")}</h4>
							<div style={{ display: "flex", gap: 24, flexWrap: "wrap" }}>
								{(() => {
									const last = this.state.liveSnapshots[this.state.liveSnapshots.length - 1];
									return (
										<>
											<div>
												<div style={{ fontSize: 18 }}>{formatBytes(last.app_heap_bytes)}</div>
												<div style={{ color: "#888" }}>{t("App heap")}</div>
											</div>
											<div>
												<div style={{ fontSize: 18 }}>{formatBytes(last.engine_overhead_bytes)}</div>
												<div style={{ color: "#888" }}>{t("Engine overhead")}</div>
											</div>
											<div>
												<div style={{ fontSize: 18 }}>{formatBytes(last.db_size_bytes)}</div>
												<div style={{ color: "#888" }}>{t("Database size (disk)")}</div>
											</div>
										</>
									);
								})()}
							</div>
						</div>
					)}
				</div>
			</div>
		);
	}
}

export default Statistics;
