import React from "react";
import PageComponent from "@engine/containers/PageComponent";
import { t, subscribe } from "@engine/controllers/i18n";
import LossChart from "./containers/LossChart";

const STORAGE_KEY = "transformerStudio.server";

interface Fields {
  target_merges: string;
  population_sample_size: string;
  d_model: string;
  num_layers: string;
  seq_len: string;
  epoch_sample_size: string;
  learning_rate: string;
  temperature: string;
}

interface Stats {
  datasetSize: number | null;
  epoch: number | null;
  srcVocab: number | null;
  tgtVocab: number | null;
  trainTps: number | null;
  inferTps: number | null;
  isTraining: boolean;
  isPopulating: boolean;
  isInitialized: boolean;
  structureLocked: boolean;
}

interface State {
  server: string;
  base: string;
  connecting: boolean;
  connected: boolean;
  error: string;
  fields: Fields;
  stats: Stats;
  loss: number[];
  progress: number;
  message: string;
  busy: string;
  predictInput: string;
  predictOutput: string;
  predictMeta: string;
}

const emptyFields: Fields = {
  target_merges: "",
  population_sample_size: "",
  d_model: "",
  num_layers: "",
  seq_len: "",
  epoch_sample_size: "",
  learning_rate: "",
  temperature: "1",
};

const emptyStats: Stats = {
  datasetSize: null,
  epoch: null,
  srcVocab: null,
  tgtVocab: null,
  trainTps: null,
  inferTps: null,
  isTraining: false,
  isPopulating: false,
  isInitialized: false,
  structureLocked: false,
};

const GRID: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 16, marginBottom: 16 };

const normalizeServer = (raw: string) => {
  let s = raw.trim().replace(/\/+$/, "");
  if (s && !/^https?:\/\//i.test(s)) s = "http://" + s;
  return s;
};

const pick = (d: any, ...keys: string[]) => {
  for (const k of keys) if (d[k] !== undefined && d[k] !== null) return d[k];
  return undefined;
};

const fieldKeys: (keyof Fields)[] = [
  "target_merges",
  "population_sample_size",
  "d_model",
  "num_layers",
  "seq_len",
  "epoch_sample_size",
  "learning_rate",
  "temperature",
];

export default class TransformerStudio extends PageComponent<{}, State> {
  protected isPage = true;
  protected href = "transformer-studio";
  protected title = "Transformer Studio";
  protected submenu = "tools";
  protected icon = "statistics";
  protected requiresAuth = true;

  private unsubscribeI18n?: () => void;
  private source?: EventSource;

  constructor(props: {}) {
    super(props);
    let server = "";
    try {
      server = localStorage.getItem(STORAGE_KEY) || "";
    } catch {
      // storage unavailable
    }
    this.state = {
      server,
      base: "",
      connecting: false,
      connected: false,
      error: "",
      fields: emptyFields,
      stats: emptyStats,
      loss: [],
      progress: 0,
      message: "",
      busy: "",
      predictInput: "",
      predictOutput: "",
      predictMeta: "",
    };
  }

  async componentDidMount() {
    await super.componentDidMount();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }

  componentWillUnmount() {
    this.unsubscribeI18n?.();
    this.source?.close();
  }

  private call = async (path: string, init?: RequestInit, base = this.state.base) => {
    const res = await fetch(`${base}/${path}`, init);
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`.trim());
    const text = await res.text();
    try {
      return text ? JSON.parse(text) : {};
    } catch {
      return { message: text };
    }
  };

  private applyStatus = (d: any, fillInputs: boolean) => {
    const num = (v: any) => (typeof v === "number" ? v : null);
    this.setState((s) => {
      const fields = { ...s.fields };
      if (fillInputs) {
        for (const k of fieldKeys) {
          const v = d[k];
          if (v !== undefined && v !== null) fields[k] = String(v);
        }
      }
      const stats: Stats = {
        ...s.stats,
        datasetSize: num(pick(d, "dataset_size")) ?? s.stats.datasetSize,
        epoch: num(pick(d, "current_epoch", "epoch")) ?? s.stats.epoch,
        srcVocab: num(pick(d, "src_vocab_size", "src_vocab_len")) ?? s.stats.srcVocab,
        tgtVocab: num(pick(d, "tgt_vocab_size", "tgt_vocab_len")) ?? s.stats.tgtVocab,
        isTraining: d.is_training ?? s.stats.isTraining,
        isPopulating: d.is_populating ?? s.stats.isPopulating,
        isInitialized: d.is_initialized ?? s.stats.isInitialized,
        structureLocked: d.structure_locked ?? s.stats.structureLocked,
      };
      return { fields, stats, loss: Array.isArray(d.loss_history) ? d.loss_history : s.loss };
    });
  };

  private openEvents = (base: string) => {
    this.source?.close();
    const es = new EventSource(`${base}/events`);
    es.onmessage = (e) => {
      let msg: any;
      try {
        msg = JSON.parse(e.data);
      } catch {
        return;
      }
      const type = msg.type;
      const d = msg.data ?? msg;
      if (type === "status") this.applyStatus(d, false);
      else if (type === "progress") {
        const p = Number(d.progress) || 0;
        this.setState((s) => ({
          progress: p <= 1 ? p * 100 : p,
          message: d.message ?? s.message,
          loss: typeof d.current_loss === "number" ? [...s.loss, d.current_loss] : s.loss,
          stats: { ...s.stats, trainTps: typeof d.tokens_per_sec === "number" ? d.tokens_per_sec : s.stats.trainTps },
        }));
      } else if (type === "epoch_complete") this.setState({ message: d.message ?? "", progress: 100 });
    };
    this.source = es;
  };

  private connect = async () => {
    const base = normalizeServer(this.state.server);
    if (!base) return;
    this.setState({ connecting: true, error: "" });
    try {
      const d = await this.call("status", undefined, base);
      if (!d || typeof d !== "object") throw new Error(t("Unexpected response"));
      try {
        localStorage.setItem(STORAGE_KEY, base);
      } catch {
        // storage unavailable
      }
      this.setState({ base, server: base, connected: true, connecting: false, progress: 0, message: "" });
      this.applyStatus(d, true);
      this.openEvents(base);
    } catch (e: any) {
      this.source?.close();
      this.setState({ connected: false, connecting: false, error: `${t("Could not connect")}: ${e?.message || e}` });
    }
  };

  private disconnect = () => {
    this.source?.close();
    this.setState({ connected: false, base: "", stats: emptyStats, loss: [], progress: 0, message: "", error: "" });
  };

  private run = async (label: string, fn: () => Promise<void>) => {
    this.setState({ busy: label, error: "" });
    try {
      await fn();
    } catch (e: any) {
      this.setState({ error: `${label}: ${e?.message || e}` });
    } finally {
      this.setState({ busy: "" });
    }
  };

  private setField = (k: keyof Fields, v: string) => this.setState((s) => ({ fields: { ...s.fields, [k]: v } }));

  private populate = () =>
    this.run(t("Populate"), async () => {
      const f = this.state.fields;
      const d = await this.call("populate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          target_merges: Number(f.target_merges),
          population_sample_size: Number(f.population_sample_size),
        }),
      });
      if (d.message) this.setState({ message: String(d.message) });
    });

  private saveConfig = () =>
    this.run(t("Save config"), async () => {
      const f = this.state.fields;
      const d = await this.call("config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          d_model: Number(f.d_model),
          num_layers: Number(f.num_layers),
          seq_len: Number(f.seq_len),
          epoch_sample_size: Number(f.epoch_sample_size),
          learning_rate: Number(f.learning_rate),
          temperature: Number(f.temperature),
        }),
      });
      if (d.message) this.setState({ message: String(d.message) });
    });

  private startTrain = () =>
    this.run(t("Start epoch"), async () => {
      this.setState({ progress: 0 });
      const d = await this.call("start-train");
      if (d.message) this.setState({ message: String(d.message) });
    });

  private predict = () =>
    this.run(t("Predict"), async () => {
      const d = await this.call(`predict?text=${encodeURIComponent(this.state.predictInput)}`);
      const meta = [
        d.tokens_per_sec !== undefined && `${Number(d.tokens_per_sec).toFixed(1)} tok/s`,
        d.generated_token_count !== undefined && `${d.generated_token_count} ${t("tokens")}`,
        d.latency_ms !== undefined && `${Number(d.latency_ms).toFixed(0)} ms`,
        d.temperature !== undefined && `T=${d.temperature}`,
      ].filter(Boolean);
      this.setState((s) => ({
        predictOutput: String(d.predicted_text ?? ""),
        predictMeta: meta.join(" · "),
        stats: { ...s.stats, inferTps: d.tokens_per_sec !== undefined ? Number(d.tokens_per_sec) : s.stats.inferTps },
      }));
    });

  private num = (label: string, k: keyof Fields, disabled: boolean, step?: string) => (
    <div>
      <div className="text-muted small text-uppercase mb-1">{label}</div>
      <input
        type="number"
        step={step}
        className="form-control"
        disabled={disabled}
        value={this.state.fields[k]}
        onChange={(e) => this.setField(k, e.target.value)}
      />
    </div>
  );

  private stat = (label: string, value: number | string | null) => (
    <div>
      <div className="text-muted small text-uppercase mb-1">{label}</div>
      <div style={{ fontSize: 20, fontWeight: 600 }}>{value === null ? "—" : value}</div>
    </div>
  );

  render() {
    const { server, connecting, connected, error, fields, stats, loss, progress, message, busy, predictInput, predictOutput, predictMeta } = this.state;
    const off = !connected || !!busy;
    const fmt = (v: number | null) => (v === null ? null : Number.isInteger(v) ? v : v.toFixed(1));

    return (
      <div className="container-fluid pt-4">
        <h4 className="mb-3">{t("Transformer Studio")}</h4>

        <div className="card mb-3">
          <div className="card-body">
            <div className="text-muted small text-uppercase mb-1">{t("Server")}</div>
            <form
              className="d-flex gap-2 flex-wrap align-items-center"
              onSubmit={(e) => {
                e.preventDefault();
                if (!connected) this.connect();
              }}
            >
              <input
                type="text"
                className="form-control"
                style={{ maxWidth: 420 }}
                placeholder="http://localhost:8080"
                disabled={connected || connecting}
                value={server}
                onChange={(e) => this.setState({ server: e.target.value })}
              />
              {connected ? (
                <button type="button" className="btn btn-outline-secondary" onClick={this.disconnect}>{t("Disconnect")}</button>
              ) : (
                <button type="submit" className="btn btn-primary" disabled={connecting || !server.trim()}>
                  {connecting ? t("Connecting…") : t("Connect")}
                </button>
              )}
              <span className={"badge " + (connected ? "bg-success" : "bg-secondary")}>
                {connected ? t("Connected") : t("Not connected")}
              </span>
              {connected && stats.isTraining && <span className="badge bg-secondary">{t("Training")}</span>}
              {connected && stats.isPopulating && <span className="badge bg-secondary">{t("Populating")}</span>}
            </form>
            {error && <div className="text-danger small mt-2">{error}</div>}
          </div>
        </div>

        <div className="card mb-3">
          <div className="card-body">
            <div style={GRID}>
              {this.stat(t("Dataset rows"), stats.datasetSize)}
              {this.stat(t("Epoch"), stats.epoch)}
              {this.stat(t("Training tok/s"), fmt(stats.trainTps))}
              {this.stat(t("Inference tok/s"), fmt(stats.inferTps))}
            </div>
            <div className="text-muted small">
              {t("Vocab")}: {stats.srcVocab ?? "—"} / {stats.tgtVocab ?? "—"}
              {" · "}
              {stats.isInitialized ? t("Model initialized") : t("Model not initialized")}
            </div>
          </div>
        </div>

        <div className="card mb-3">
          <div className="card-body">
            <div className="text-muted small text-uppercase mb-2">{t("Dataset population")}</div>
            <div style={GRID}>
              {this.num(t("Target merges"), "target_merges", off)}
              {this.num(t("Population sample size"), "population_sample_size", off)}
            </div>
            <button type="button" className="btn btn-primary" disabled={off || stats.isPopulating} onClick={this.populate}>
              {t("Populate")}
            </button>
          </div>
        </div>

        <div className="card mb-3">
          <div className="card-body">
            <div className="text-muted small text-uppercase mb-2">{t("Hyperparameters")}</div>
            <div style={GRID}>
              {this.num("d_model", "d_model", off || stats.structureLocked)}
              {this.num(t("Layers"), "num_layers", off || stats.structureLocked)}
              {this.num(t("Sequence length"), "seq_len", off || stats.structureLocked)}
              {this.num(t("Epoch sample size"), "epoch_sample_size", off)}
              {this.num(t("Learning rate"), "learning_rate", off, "any")}
              <div style={{ gridColumn: "span 2" }}>
                <div className="text-muted small text-uppercase mb-1">{t("Temperature")}: {fields.temperature}</div>
                <input
                  type="range"
                  min="0.1"
                  max="2"
                  step="0.05"
                  className="form-control"
                  disabled={off}
                  value={fields.temperature || "1"}
                  onChange={(e) => this.setField("temperature", e.target.value)}
                />
              </div>
            </div>
            <button type="button" className="btn btn-outline-secondary" disabled={off} onClick={this.saveConfig}>
              {t("Save config")}
            </button>
          </div>
        </div>

        <div className="card mb-3">
          <div className="card-body">
            <div className="d-flex gap-2 flex-wrap align-items-center mb-3">
              <div className="text-muted small text-uppercase">{t("Training")}</div>
              <button type="button" className="btn btn-primary" disabled={off || stats.isTraining} onClick={this.startTrain}>
                {t("Start epoch")}
              </button>
              <span className="text-muted small">{message}</span>
            </div>
            <div style={{ height: 8, borderRadius: 4, background: "rgba(128,128,128,0.25)", overflow: "hidden", marginBottom: 12 }}>
              <div style={{ width: `${Math.min(100, Math.max(0, progress))}%`, height: "100%", background: "#0d6efd", transition: "width .3s" }} />
            </div>
            <div className="text-muted small text-uppercase mb-1">{t("Loss")}</div>
            <LossChart values={loss} />
          </div>
        </div>

        <div className="card mb-3">
          <div className="card-body">
            <div className="text-muted small text-uppercase mb-1">{t("Inference")}</div>
            <form
              className="d-flex gap-2 mb-2"
              onSubmit={(e) => {
                e.preventDefault();
                if (!off && predictInput.trim()) this.predict();
              }}
            >
              <input
                type="text"
                className="form-control"
                disabled={off}
                value={predictInput}
                onChange={(e) => this.setState({ predictInput: e.target.value })}
              />
              <button type="submit" className="btn btn-primary" disabled={off || !predictInput.trim()}>{t("Predict")}</button>
            </form>
            {predictOutput && <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>{predictOutput}</pre>}
            {predictMeta && <div className="text-muted small mt-1">{predictMeta}</div>}
          </div>
        </div>
      </div>
    );
  }
}
