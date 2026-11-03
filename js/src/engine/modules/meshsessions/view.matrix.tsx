import React, { useEffect, useMemo, useState } from "react";
import { ModuleViewProps } from "@engine/controllers/registry";
import useT from "@engine/controllers/useT";

const REFRESH_MS = 3000;

type MatrixCell = {
    latencyMs?: number;
    upMbps?: number;
    measured: boolean;
    sends?: string[];
};

type RoomMatrix = {
    room: string;
    peers: string[];
    retranslator?: string;
    matrix: MatrixCell[][];
};

type TimelinePoint = {
    time: string;
    matrix: RoomMatrix;
};

type RoomMatrixRecord = RoomMatrix & {
    history?: TimelinePoint[];
};

function peerLabel(id: string, retranslator?: string): string {
    return id === retranslator ? `Server (${id})` : id;
}

function cellStyle(isRetranslatorRow: boolean, isRetranslatorCol: boolean): React.CSSProperties {
    if (isRetranslatorRow || isRetranslatorCol) {
        return { background: "var(--bs-warning-bg-subtle, #fff8e1)" };
    }
    return {};
}

// Custom VIEW mode "matrix": one room's full peer-to-peer adjacency matrix
// (go/engine/controllers/mesh.Coordinator.RoomMatrix) — every reporting peer
// as both a row and column, each cell showing what that row peer is
// actually forwarding to the column peer right now plus its own
// self-measured link speed/latency to it. The server's own retranslator
// peer ("0") appears as an ordinary row/column whenever the coordinator has
// it engaged for this room, highlighted rather than hidden. A timeline
// slider (mesh.Coordinator.Timeline's recorded history) lets the viewer
// scrub back through how the room's routing evolved, instead of only ever
// seeing the live state.
const MeshMatrixView: React.FC<ModuleViewProps> = ({ record, reload }) => {
    const t = useT();
    const [now, setNow] = useState(() => Date.now());
    const [scrubIndex, setScrubIndex] = useState<number | null>(null); // null = follow live

    useEffect(() => {
        const timer = setInterval(() => {
            reload();
            setNow(Date.now());
        }, REFRESH_MS);
        return () => clearInterval(timer);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const rec = record as RoomMatrixRecord | undefined;
    const history = rec?.history ?? [];
    const liveIndex = history.length - 1;
    const isLive = scrubIndex === null || scrubIndex >= liveIndex;
    const shownIndex = isLive ? liveIndex : Math.max(0, Math.min(scrubIndex!, liveIndex));
    const activePoint = !isLive && history.length > 0 ? history[shownIndex] : null;

    // Live record fields are always the freshest fetch (RoomMatrix computed
    // fresh on every load), so prefer them while following live; only a
    // scrubbed-back point substitutes its own recorded peers/matrix.
    const peers = activePoint ? activePoint.matrix.peers : rec?.peers ?? [];
    const matrix = activePoint ? activePoint.matrix.matrix : rec?.matrix ?? [];
    const retranslator = activePoint ? activePoint.matrix.retranslator : rec?.retranslator;
    const hasRetranslator = !!retranslator;

    const cellLabel = (cell: MatrixCell | undefined): React.ReactNode => {
        if (!cell) return <span className="text-muted">—</span>;
        const parts: React.ReactNode[] = [];
        if (cell.sends && cell.sends.length > 0) {
            parts.push(
                <div key="sends" style={{ fontWeight: 600 }}>
                    → {cell.sends.join(", ")}
                </div>
            );
        }
        if (cell.measured) {
            parts.push(
                <div key="measured" className="text-muted" style={{ fontSize: 11 }}>
                    {(cell.latencyMs ?? 0).toFixed(0)}ms · {(cell.upMbps ?? 0).toFixed(1)}Mbps
                </div>
            );
        }
        if (parts.length === 0) return <span className="text-muted">·</span>;
        return <>{parts}</>;
    };

    const empty = useMemo(() => peers.length === 0, [peers]);

    if (empty) {
        return (
            <div className="p-3">
                <div className="text-muted">{t("No live session data for this room.")}</div>
            </div>
        );
    }

    return (
        <div style={{ padding: "1rem" }}>
            <div className="d-flex justify-content-between align-items-center mb-2">
                <div>
                    <span className="fw-semibold">{rec?.room}</span>
                    {hasRetranslator && (
                        <span className="badge bg-warning text-dark ms-2">{t("retranslator active")}</span>
                    )}
                    {!isLive && <span className="badge bg-secondary ms-2">{t("replay")}</span>}
                </div>
                <span className="text-muted" style={{ fontSize: 12 }}>
                    {t("Auto-refreshing every")} {REFRESH_MS / 1000}s · {new Date(now).toLocaleTimeString()}
                </span>
            </div>

            {history.length > 1 && (
                <div className="mb-3 px-1">
                    <input
                        type="range"
                        className="form-range"
                        min={0}
                        max={liveIndex}
                        value={shownIndex}
                        onChange={(e) => {
                            const v = Number(e.target.value);
                            setScrubIndex(v >= liveIndex ? null : v);
                        }}
                    />
                    <div className="d-flex justify-content-between align-items-center" style={{ fontSize: 12 }}>
                        <span className="text-muted">{new Date(history[0].time).toLocaleTimeString()}</span>
                        <span className="fw-semibold">
                            {isLive ? t("live") : new Date(history[shownIndex].time).toLocaleTimeString()}
                            {!isLive && (
                                <button
                                    className="btn btn-sm btn-link p-0 ms-2"
                                    onClick={() => setScrubIndex(null)}
                                >
                                    {t("Jump to live")}
                                </button>
                            )}
                        </span>
                        <span className="text-muted">{new Date(history[liveIndex].time).toLocaleTimeString()}</span>
                    </div>
                </div>
            )}

            <div className="table-responsive">
                <table className="table table-sm table-bordered mb-0 align-middle" style={{ fontSize: 12 }}>
                    <thead>
                        <tr>
                            <th style={{ background: "var(--bs-secondary-bg, #e9ecef)" }}>{t("from \\ to")}</th>
                            {peers.map((p) => (
                                <th
                                    key={p}
                                    style={p === retranslator ? { background: "var(--bs-warning-bg-subtle, #fff8e1)" } : undefined}
                                >
                                    {peerLabel(p, retranslator)}
                                </th>
                            ))}
                        </tr>
                    </thead>
                    <tbody>
                        {peers.map((rowPeer, i) => (
                            <tr key={rowPeer}>
                                <th style={rowPeer === retranslator ? { background: "var(--bs-warning-bg-subtle, #fff8e1)" } : undefined}>
                                    {peerLabel(rowPeer, retranslator)}
                                </th>
                                {peers.map((colPeer, j) => (
                                    <td
                                        key={colPeer}
                                        style={cellStyle(rowPeer === retranslator, colPeer === retranslator)}
                                    >
                                        {rowPeer === colPeer ? (
                                            <span className="text-muted">—</span>
                                        ) : (
                                            cellLabel(matrix[i]?.[j])
                                        )}
                                    </td>
                                ))}
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
};

export default MeshMatrixView;
