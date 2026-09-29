import React, { useState } from "react";
import { t } from "@engine/controllers/i18n";
import ImageDrop from "./ImageDrop";
import { BlobResult } from "./api";

interface ImageDefaults {
  width?: number;
  height?: number;
  steps?: number;
  guidance?: number;
  t5_option?: boolean;
  img2img?: boolean;
}

const ImagesPanel: React.FC<{
  defaults: ImageDefaults;
  busy: boolean;
  onGenerate: (params: Record<string, unknown>) => Promise<BlobResult>;
}> = ({ defaults, busy, onGenerate }) => {
  const [width, setWidth] = useState(String(defaults.width ?? 512));
  const [height, setHeight] = useState(String(defaults.height ?? 512));
  const [steps, setSteps] = useState(String(defaults.steps ?? 28));
  const [guidance, setGuidance] = useState(String(defaults.guidance ?? 4));
  const [seed, setSeed] = useState("");
  const [useT5, setUseT5] = useState(defaults.t5_option ?? true);
  const [initImage, setInitImage] = useState<string | null>(null);
  const [strength, setStrength] = useState("0.6");
  const [prompt, setPrompt] = useState("");

  const [result, setResult] = useState<BlobResult | null>(null);
  const [error, setError] = useState("");

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    const params: Record<string, unknown> = {
      prompt,
      width: Number(width),
      height: Number(height),
      steps: Number(steps),
      guidance: Number(guidance),
      use_t5: useT5,
    };
    if (seed.trim()) params.seed = Number(seed);
    if (initImage) {
      params.init_image = initImage;
      params.strength = Number(strength);
    }
    try {
      setResult(await onGenerate(params));
    } catch (err: any) {
      setError(err?.message || String(err));
    }
  };

  return (
    <div className="card mb-3">
      <div className="card-body">
        <div className="text-muted small text-uppercase mb-2">{t("Text to image")}</div>
        <form onSubmit={submit}>
          <div className="mb-3">
            <div className="text-muted small text-uppercase mb-1">{t("Prompt")}</div>
            <textarea className="form-control" rows={2} disabled={busy} value={prompt} onChange={(e) => setPrompt(e.target.value)} />
          </div>
          <div className="row">
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Width")}</div>
              <input type="number" step={32} className="form-control" disabled={busy} value={width} onChange={(e) => setWidth(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Height")}</div>
              <input type="number" step={32} className="form-control" disabled={busy} value={height} onChange={(e) => setHeight(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Steps")}</div>
              <input type="number" className="form-control" disabled={busy} value={steps} onChange={(e) => setSteps(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Guidance")}</div>
              <input type="number" step="0.1" className="form-control" disabled={busy} value={guidance} onChange={(e) => setGuidance(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Seed")}</div>
              <input type="number" className="form-control" disabled={busy} placeholder={t("random")} value={seed} onChange={(e) => setSeed(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3 d-flex align-items-end">
              <div className="form-check">
                <input type="checkbox" className="form-check-input" id="use_t5" disabled={busy} checked={useT5} onChange={(e) => setUseT5(e.target.checked)} />
                <label className="form-check-label small" htmlFor="use_t5">{t("Use T5 (SD3)")}</label>
              </div>
            </div>
          </div>

          <ImageDrop label={t("Start image (optional, image-to-image)")} disabled={busy} onChange={setInitImage} />
          {initImage && (
            <div className="mt-2" style={{ maxWidth: 240 }}>
              <div className="text-muted small text-uppercase mb-1">{t("Strength")}: {strength}</div>
              <input type="range" min="0" max="1" step="0.05" className="form-control" disabled={busy} value={strength} onChange={(e) => setStrength(e.target.value)} />
            </div>
          )}

          <button type="submit" className="btn btn-primary mt-3" disabled={busy || !prompt.trim()}>
            {busy ? t("Generating…") : t("Generate")}
          </button>
        </form>

        {error && <div className="text-danger small mt-2">{error}</div>}
        {result && (
          <div className="mt-3">
            <img src={result.url} alt="" style={{ maxWidth: "100%", borderRadius: 6 }} />
            <div className="text-muted small mt-1">
              {result.headers["X-Seed"] && `${t("Seed")}: ${result.headers["X-Seed"]} · `}
              {result.headers["X-Generation-Ms"] && `${Number(result.headers["X-Generation-Ms"]).toFixed(0)} ms`}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default ImagesPanel;
