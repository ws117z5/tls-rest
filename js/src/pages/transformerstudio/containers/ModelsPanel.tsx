import React, { useState } from "react";
import { t } from "@engine/controllers/i18n";

export interface ModelEntry {
  name: string;
  kind: string;
  loadable: boolean;
  reason?: string;
  selected?: boolean;
  has_weights?: boolean;
  size_bytes?: number;
  params?: import("./RangeControl").ParamSpec[]; // enhancer only
}

const sizeOf = (bytes?: number) => {
  if (!bytes) return "";
  const mb = bytes / (1024 * 1024);
  return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${mb.toFixed(0)} MB`;
};

const ModelsPanel: React.FC<{
  models: ModelEntry[];
  busy: boolean;
  onSelect: (name: string) => void;
  onCreate: (params: Record<string, unknown>) => void;
}> = ({ models, busy, onSelect, onCreate }) => {
  const [name, setName] = useState("");
  const [tokenizer, setTokenizer] = useState("");
  const [dModel, setDModel] = useState("512");
  const [numLayers, setNumLayers] = useState("4");
  const [batchSize, setBatchSize] = useState("128");
  const [learningRate, setLearningRate] = useState("");

  const create = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !tokenizer.trim()) return;
    const params: Record<string, unknown> = {
      name: name.trim(),
      tokenizer: tokenizer.trim(),
      d_model: Number(dModel),
      num_layers: Number(numLayers),
      batch_size: Number(batchSize),
    };
    if (learningRate.trim()) params.learning_rate = Number(learningRate);
    onCreate(params);
    setName("");
  };

  return (
    <div className="card mb-3">
      <div className="card-body">
        <div className="text-muted small text-uppercase mb-2">{t("Models")}</div>
        <table className="table table-sm align-middle">
          <thead>
            <tr>
              <th>{t("Name")}</th>
              <th>{t("Kind")}</th>
              <th>{t("Size")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {models.map((m) => (
              <tr key={m.name}>
                <td>{m.name}</td>
                <td>
                  <span className="badge bg-secondary-subtle text-body">{m.kind}</span>
                </td>
                <td className="text-muted small">{sizeOf(m.size_bytes)}</td>
                <td className="text-end">
                  {m.selected ? (
                    <span className="badge bg-success">{t("Selected")}</span>
                  ) : m.loadable ? (
                    <button type="button" className="btn btn-outline-primary btn-sm" disabled={busy} onClick={() => onSelect(m.name)}>
                      {t("Select")}
                    </button>
                  ) : (
                    <span className="text-muted small" title={m.reason}>
                      {t("Unavailable")}
                    </span>
                  )}
                </td>
              </tr>
            ))}
            {models.length === 0 && (
              <tr>
                <td colSpan={4} className="text-muted">
                  {t("No models found.")}
                </td>
              </tr>
            )}
          </tbody>
        </table>

        <div className="text-muted small text-uppercase mt-3 mb-2">{t("Create a native model")}</div>
        <form className="row g-2 align-items-end" onSubmit={create}>
          <div className="col-6 col-md-3">
            <div className="text-muted small mb-1">{t("Name")}</div>
            <input className="form-control" disabled={busy} value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="col-6 col-md-3">
            <div className="text-muted small mb-1">{t("Tokenizer set")}</div>
            <input className="form-control" disabled={busy} value={tokenizer} onChange={(e) => setTokenizer(e.target.value)} />
          </div>
          <div className="col-4 col-md-2">
            <div className="text-muted small mb-1">d_model</div>
            <input type="number" className="form-control" disabled={busy} value={dModel} onChange={(e) => setDModel(e.target.value)} />
          </div>
          <div className="col-4 col-md-1">
            <div className="text-muted small mb-1">{t("Layers")}</div>
            <input type="number" className="form-control" disabled={busy} value={numLayers} onChange={(e) => setNumLayers(e.target.value)} />
          </div>
          <div className="col-4 col-md-2">
            <div className="text-muted small mb-1">{t("Batch size")}</div>
            <input type="number" className="form-control" disabled={busy} value={batchSize} onChange={(e) => setBatchSize(e.target.value)} />
          </div>
          <div className="col-6 col-md-2">
            <div className="text-muted small mb-1">{t("Learning rate")}</div>
            <input className="form-control" disabled={busy} placeholder={t("default")} value={learningRate} onChange={(e) => setLearningRate(e.target.value)} />
          </div>
          <div className="col-12">
            <button type="submit" className="btn btn-primary" disabled={busy || !name.trim() || !tokenizer.trim()}>
              {t("Create")}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default ModelsPanel;
