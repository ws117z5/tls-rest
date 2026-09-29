import React from "react";
import { t } from "@engine/controllers/i18n";

interface StudioStatus {
  model?: string;
  kind?: string;
  is_training?: boolean;
  is_populating?: boolean;
  is_generating?: boolean;
}

const ServerBar: React.FC<{
  server: string;
  onServerChange: (v: string) => void;
  connected: boolean;
  connecting: boolean;
  error: string;
  status: StudioStatus | null;
  onConnect: () => void;
  onDisconnect: () => void;
}> = ({ server, onServerChange, connected, connecting, error, status, onConnect, onDisconnect }) => (
  <div className="card mb-3">
    <div className="card-body">
      <div className="text-muted small text-uppercase mb-1">{t("Server")}</div>
      <form
        className="d-flex gap-2 flex-wrap align-items-center"
        onSubmit={(e) => {
          e.preventDefault();
          if (!connected) onConnect();
        }}
      >
        <input
          type="text"
          className="form-control"
          style={{ maxWidth: 420 }}
          placeholder="http://localhost:8080"
          disabled={connected || connecting}
          value={server}
          onChange={(e) => onServerChange(e.target.value)}
        />
        {connected ? (
          <button type="button" className="btn btn-outline-secondary" onClick={onDisconnect}>
            {t("Disconnect")}
          </button>
        ) : (
          <button type="submit" className="btn btn-primary" disabled={connecting || !server.trim()}>
            {connecting ? t("Connecting…") : t("Connect")}
          </button>
        )}
        <span className={"badge " + (connected ? "bg-success" : "bg-secondary")}>
          {connected ? t("Connected") : t("Not connected")}
        </span>
        {connected && status?.is_training && <span className="badge bg-secondary">{t("Training")}</span>}
        {connected && status?.is_populating && <span className="badge bg-secondary">{t("Building tokenizer")}</span>}
        {connected && status?.is_generating && <span className="badge bg-secondary">{t("Generating")}</span>}
      </form>
      {error && <div className="text-danger small mt-2">{error}</div>}
      {connected && status && (
        <div className="text-muted small mt-2">
          {t("Model")}: {status.model || "—"} ({status.kind || "—"})
        </div>
      )}
    </div>
  </div>
);

export default ServerBar;
