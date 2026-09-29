import React, { useEffect, useState } from "react";
import { t } from "@engine/controllers/i18n";
import LossChart from "./LossChart";

interface StudioStatus {
  is_training?: boolean;
  is_populating?: boolean;
  structure_locked?: boolean;
  seq_len?: number;
  d_model?: number;
  num_layers?: number;
  batch_size?: number;
  learning_rate?: number;
  temperature?: number;
  epoch_sample_size?: number;
  current_epoch?: number;
  src_vocab_size?: number;
  tgt_vocab_size?: number;
  dataset_size?: number;
}

const TrainPanel: React.FC<{
  status: StudioStatus;
  lossHistory: number[];
  progress: number;
  message: string;
  busy: string;
  onBuildTokenizer: (name: string, targetMerges: number, populationSampleSize: number) => void;
  onSaveConfig: (params: Record<string, unknown>) => void;
  onStart: () => void;
  onStop: () => void;
}> = ({ status, lossHistory, progress, message, busy, onBuildTokenizer, onSaveConfig, onStart, onStop }) => {
  const [tokenizerName, setTokenizerName] = useState("");
  const [targetMerges, setTargetMerges] = useState("3000");
  const [populationSampleSize, setPopulationSampleSize] = useState("100000");

  const [dModel, setDModel] = useState(String(status.d_model ?? ""));
  const [numLayers, setNumLayers] = useState(String(status.num_layers ?? ""));
  const [batchSize, setBatchSize] = useState(String(status.batch_size ?? ""));
  const [learningRate, setLearningRate] = useState(String(status.learning_rate ?? ""));
  const [epochSampleSize, setEpochSampleSize] = useState(String(status.epoch_sample_size ?? ""));
  const [temperature, setTemperature] = useState(String(status.temperature ?? 1));

  // Re-sync the form when the server's own values change (e.g. after Save, or a fresh /status).
  useEffect(() => setDModel(String(status.d_model ?? "")), [status.d_model]);
  useEffect(() => setNumLayers(String(status.num_layers ?? "")), [status.num_layers]);
  useEffect(() => setBatchSize(String(status.batch_size ?? "")), [status.batch_size]);
  useEffect(() => setLearningRate(String(status.learning_rate ?? "")), [status.learning_rate]);
  useEffect(() => setEpochSampleSize(String(status.epoch_sample_size ?? "")), [status.epoch_sample_size]);
  useEffect(() => setTemperature(String(status.temperature ?? 1)), [status.temperature]);

  const off = !!busy;
  const structureLocked = !!status.structure_locked;

  return (
    <>
      <div className="card mb-3">
        <div className="card-body">
          <div className="row">
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Epoch")}</div>
              <div style={{ fontSize: 20, fontWeight: 600 }}>{status.current_epoch ?? "—"}</div>
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Dataset rows")}</div>
              <div style={{ fontSize: 20, fontWeight: 600 }}>{status.dataset_size ?? "—"}</div>
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Vocab (src/tgt)")}</div>
              <div style={{ fontSize: 20, fontWeight: 600 }}>
                {status.src_vocab_size ?? "—"}/{status.tgt_vocab_size ?? "—"}
              </div>
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Sequence length")}</div>
              <div style={{ fontSize: 20, fontWeight: 600 }}>{status.seq_len ?? "—"}</div>
            </div>
          </div>
        </div>
      </div>

      <div className="card mb-3">
        <div className="card-body">
          <div className="text-muted small text-uppercase mb-2">{t("Tokenizer")}</div>
          <form
            className="row g-2 align-items-end"
            onSubmit={(e) => {
              e.preventDefault();
              if (tokenizerName.trim()) onBuildTokenizer(tokenizerName.trim(), Number(targetMerges), Number(populationSampleSize));
            }}
          >
            <div className="col-6 col-md-4">
              <div className="text-muted small mb-1">{t("New tokenizer name")}</div>
              <input className="form-control" disabled={off} value={tokenizerName} onChange={(e) => setTokenizerName(e.target.value)} />
            </div>
            <div className="col-6 col-md-4">
              <div className="text-muted small mb-1">{t("Target merges")}</div>
              <input type="number" className="form-control" disabled={off} value={targetMerges} onChange={(e) => setTargetMerges(e.target.value)} />
            </div>
            <div className="col-6 col-md-4">
              <div className="text-muted small mb-1">{t("Population sample size")}</div>
              <input type="number" className="form-control" disabled={off} value={populationSampleSize} onChange={(e) => setPopulationSampleSize(e.target.value)} />
            </div>
            <div className="col-12">
              <button type="submit" className="btn btn-outline-primary" disabled={off || status.is_populating || !tokenizerName.trim()}>
                {status.is_populating ? t("Building…") : t("Build tokenizer")}
              </button>
            </div>
          </form>
        </div>
      </div>

      <div className="card mb-3">
        <div className="card-body">
          <div className="text-muted small text-uppercase mb-2">{t("Hyperparameters")}</div>
          <div className="row">
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">d_model</div>
              <input type="number" className="form-control" disabled={off || structureLocked} value={dModel} onChange={(e) => setDModel(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Layers")}</div>
              <input type="number" className="form-control" disabled={off || structureLocked} value={numLayers} onChange={(e) => setNumLayers(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Batch size")}</div>
              <input type="number" className="form-control" disabled={off} value={batchSize} onChange={(e) => setBatchSize(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Learning rate")}</div>
              <input className="form-control" disabled={off} value={learningRate} onChange={(e) => setLearningRate(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Epoch sample size")}</div>
              <input type="number" className="form-control" disabled={off} value={epochSampleSize} onChange={(e) => setEpochSampleSize(e.target.value)} />
            </div>
            <div className="col-6 col-md-6 mb-3">
              <div className="text-muted small text-uppercase mb-1">
                {t("Temperature")}: {temperature} {Number(temperature) === 0 && `(${t("greedy")})`}
              </div>
              <input
                type="range"
                min="0"
                max="2"
                step="0.05"
                className="form-control"
                disabled={off}
                value={temperature}
                onChange={(e) => setTemperature(e.target.value)}
              />
            </div>
          </div>
          <button
            type="button"
            className="btn btn-outline-secondary"
            disabled={off}
            onClick={() =>
              onSaveConfig({
                d_model: Number(dModel),
                num_layers: Number(numLayers),
                batch_size: Number(batchSize),
                learning_rate: Number(learningRate),
                epoch_sample_size: Number(epochSampleSize),
                temperature: Number(temperature),
              })
            }
          >
            {t("Save config")}
          </button>
        </div>
      </div>

      <div className="card mb-3">
        <div className="card-body">
          <div className="d-flex gap-2 flex-wrap align-items-center mb-3">
            <div className="text-muted small text-uppercase">{t("Training")}</div>
            {status.is_training ? (
              <button type="button" className="btn btn-outline-danger" disabled={off} onClick={onStop}>
                {t("Stop epoch")}
              </button>
            ) : (
              <button type="button" className="btn btn-primary" disabled={off} onClick={onStart}>
                {t("Start epoch")}
              </button>
            )}
            <span className="text-muted small">{message}</span>
          </div>
          <div style={{ height: 8, borderRadius: 4, background: "rgba(128,128,128,0.25)", overflow: "hidden", marginBottom: 12 }}>
            <div style={{ width: `${Math.min(100, Math.max(0, progress))}%`, height: "100%", background: "#0d6efd", transition: "width .3s" }} />
          </div>
          <div className="text-muted small text-uppercase mb-1">{t("Loss")}</div>
          <LossChart values={lossHistory} />
        </div>
      </div>
    </>
  );
};

export default TrainPanel;
