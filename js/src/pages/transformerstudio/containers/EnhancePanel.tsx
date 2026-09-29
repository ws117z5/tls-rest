import React, { useEffect, useState } from "react";
import { t } from "@engine/controllers/i18n";
import ImageDrop from "./ImageDrop";
import RangeControl, { ParamSpec } from "./RangeControl";
import { BlobResult } from "./api";

const EnhancePanel: React.FC<{
  params: ParamSpec[];
  busyImage: boolean;
  busyVideo: boolean;
  onEnhanceImage: (image: string, values: Record<string, number>) => Promise<BlobResult>;
  onEnhanceVideo: (file: File, values: Record<string, number>, maxFrames: number, varyNoise: boolean) => Promise<BlobResult>;
}> = ({ params, busyImage, busyVideo, onEnhanceImage, onEnhanceVideo }) => {
  const [values, setValues] = useState<Record<string, number>>({});
  useEffect(() => {
    const initial: Record<string, number> = {};
    params.forEach((p) => (initial[p.key] = p.default ?? 0));
    setValues(initial);
  }, [params]);

  const [image, setImage] = useState<string | null>(null);
  const [imageResult, setImageResult] = useState<BlobResult | null>(null);
  const [imageError, setImageError] = useState("");

  const [videoFile, setVideoFile] = useState<File | null>(null);
  const [maxFrames, setMaxFrames] = useState("0");
  const [varyNoise, setVaryNoise] = useState(false);
  const [videoResult, setVideoResult] = useState<BlobResult | null>(null);
  const [videoError, setVideoError] = useState("");

  const runImage = async () => {
    if (!image) return;
    setImageError("");
    try {
      setImageResult(await onEnhanceImage(image, values));
    } catch (e: any) {
      setImageError(e?.message || String(e));
    }
  };

  const runVideo = async () => {
    if (!videoFile) return;
    setVideoError("");
    try {
      setVideoResult(await onEnhanceVideo(videoFile, values, Number(maxFrames), varyNoise));
    } catch (e: any) {
      setVideoError(e?.message || String(e));
    }
  };

  return (
    <>
      <div className="card mb-3">
        <div className="card-body">
          <div className="text-muted small text-uppercase mb-2">{t("Controls")}</div>
          <div className="row">
            {params.map((p) => (
              <RangeControl key={p.key} spec={p} value={values[p.key] ?? p.default ?? 0} disabled={busyImage || busyVideo} onChange={(v) => setValues((s) => ({ ...s, [p.key]: v }))} />
            ))}
          </div>
        </div>
      </div>

      <div className="card mb-3">
        <div className="card-body">
          <div className="text-muted small text-uppercase mb-2">{t("Enhance an image")}</div>
          <ImageDrop label={t("Image")} disabled={busyImage} onChange={setImage} />
          <button type="button" className="btn btn-primary mt-3" disabled={busyImage || !image} onClick={runImage}>
            {busyImage ? t("Enhancing…") : t("Enhance")}
          </button>
          {imageError && <div className="text-danger small mt-2">{imageError}</div>}
          {imageResult && (
            <div className="mt-3">
              <img src={imageResult.url} alt="" style={{ maxWidth: "100%", borderRadius: 6 }} />
              {imageResult.headers["X-Enhance-Ms"] && (
                <div className="text-muted small mt-1">{Number(imageResult.headers["X-Enhance-Ms"]).toFixed(0)} ms</div>
              )}
            </div>
          )}
        </div>
      </div>

      <div className="card mb-3">
        <div className="card-body">
          <div className="text-muted small text-uppercase mb-2">{t("Enhance a video")}</div>
          <input
            type="file"
            accept="video/mp4,video/quicktime,video/x-m4v"
            className="form-control mb-2"
            disabled={busyVideo}
            onChange={(e) => setVideoFile(e.target.files?.[0] || null)}
          />
          <div className="d-flex gap-3 align-items-end flex-wrap mb-2">
            <div>
              <div className="text-muted small text-uppercase mb-1">{t("Max frames (0 = all)")}</div>
              <input type="number" className="form-control" style={{ width: 140 }} disabled={busyVideo} value={maxFrames} onChange={(e) => setMaxFrames(e.target.value)} />
            </div>
            <div className="form-check">
              <input type="checkbox" className="form-check-input" id="vary_noise" disabled={busyVideo} checked={varyNoise} onChange={(e) => setVaryNoise(e.target.checked)} />
              <label className="form-check-label small" htmlFor="vary_noise">{t("Vary noise per frame")}</label>
            </div>
          </div>
          <button type="button" className="btn btn-primary" disabled={busyVideo || !videoFile} onClick={runVideo}>
            {busyVideo ? t("Enhancing…") : t("Enhance video")}
          </button>
          {videoError && <div className="text-danger small mt-2">{videoError}</div>}
          {videoResult && (
            <div className="mt-3">
              <video src={videoResult.url} controls style={{ maxWidth: "100%", borderRadius: 6 }} />
              <div className="text-muted small mt-1">
                {videoResult.headers["X-Frames"] && `${videoResult.headers["X-Frames"]} ${t("frames")} · `}
                {videoResult.headers["X-Enhance-Ms"] && `${Number(videoResult.headers["X-Enhance-Ms"]).toFixed(0)} ms`}
              </div>
            </div>
          )}
        </div>
      </div>
    </>
  );
};

export default EnhancePanel;
