import React, { useState } from "react";
import { t } from "@engine/controllers/i18n";
import ImageDrop from "./ImageDrop";
import { BlobResult } from "./api";

interface VideoInfo {
  fps?: number;
  defaults?: { width?: number; height?: number };
}

const VideoToolPanel: React.FC<{
  available: boolean;
  info: VideoInfo | null;
  busy: boolean;
  onGenerate: (params: Record<string, unknown>) => Promise<BlobResult>;
}> = ({ available, info, busy, onGenerate }) => {
  const [image, setImage] = useState<string | null>(null);
  const [prompt, setPrompt] = useState("");
  const [negativePrompt, setNegativePrompt] = useState("");
  const [width, setWidth] = useState(String(info?.defaults?.width ?? 832));
  const [height, setHeight] = useState(String(info?.defaults?.height ?? 480));
  const [guidance, setGuidance] = useState("5");
  const [seed, setSeed] = useState("");
  const [weights, setWeights] = useState<"q8" | "f16">("q8");

  const [result, setResult] = useState<BlobResult | null>(null);
  const [error, setError] = useState("");

  if (!available) {
    return (
      <div className="card mb-3">
        <div className="card-body text-muted">{t("The image-to-video model is not installed on this server.")}</div>
      </div>
    );
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!image || !prompt.trim()) return;
    setError("");
    const params: Record<string, unknown> = {
      image,
      prompt: prompt.trim(),
      width: Number(width),
      height: Number(height),
      guidance: Number(guidance),
      weights,
    };
    if (negativePrompt.trim()) params.negative_prompt = negativePrompt.trim();
    if (seed.trim()) params.seed = Number(seed);
    try {
      setResult(await onGenerate(params));
    } catch (err: any) {
      setError(err?.message || String(err));
    }
  };

  return (
    <div className="card mb-3">
      <div className="card-body">
        <div className="text-muted small text-uppercase mb-2">{t("Image to video")}</div>
        <form onSubmit={submit}>
          <ImageDrop label={t("Start frame")} disabled={busy} onChange={setImage} />
          <div className="mb-3 mt-3">
            <div className="text-muted small text-uppercase mb-1">{t("Prompt")}</div>
            <textarea className="form-control" rows={2} disabled={busy} value={prompt} onChange={(e) => setPrompt(e.target.value)} />
          </div>
          <div className="mb-3">
            <div className="text-muted small text-uppercase mb-1">{t("Negative prompt")}</div>
            <textarea className="form-control" rows={1} disabled={busy} value={negativePrompt} onChange={(e) => setNegativePrompt(e.target.value)} />
          </div>
          <div className="row">
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Width")}</div>
              <input type="number" className="form-control" disabled={busy} value={width} onChange={(e) => setWidth(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Height")}</div>
              <input type="number" className="form-control" disabled={busy} value={height} onChange={(e) => setHeight(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Guidance")}</div>
              <input type="number" step="0.1" className="form-control" disabled={busy} value={guidance} onChange={(e) => setGuidance(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Seed")}</div>
              <input type="number" className="form-control" disabled={busy} placeholder={t("random")} value={seed} onChange={(e) => setSeed(e.target.value)} />
            </div>
            <div className="col-6 col-md-3 mb-3">
              <div className="text-muted small text-uppercase mb-1">{t("Weights")}</div>
              <select className="form-control" disabled={busy} value={weights} onChange={(e) => setWeights(e.target.value as "q8" | "f16")}>
                <option value="q8">q8</option>
                <option value="f16">f16</option>
              </select>
            </div>
          </div>
          <button type="submit" className="btn btn-primary" disabled={busy || !image || !prompt.trim()}>
            {busy ? t("Generating…") : t("Generate video")}
          </button>
        </form>
        {error && <div className="text-danger small mt-2">{error}</div>}
        {result && (
          <div className="mt-3">
            <video src={result.url} controls style={{ maxWidth: "100%", borderRadius: 6 }} />
            <div className="text-muted small mt-1">
              {result.headers["X-Seed"] && `${t("Seed")}: ${result.headers["X-Seed"]} · `}
              {result.headers["X-Frames"] && `${result.headers["X-Frames"]} ${t("frames")} · `}
              {result.headers["X-Generation-Ms"] && `${Number(result.headers["X-Generation-Ms"]).toFixed(0)} ms`}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default VideoToolPanel;
