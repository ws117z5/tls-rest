import React, { useState } from "react";
import { t } from "@engine/controllers/i18n";
import ImageDrop from "./ImageDrop";
import RangeControl, { ParamSpec } from "./RangeControl";
import { BlobResult } from "./api";

// Fixed slider set from the LivePortrait editor's own API (not model-supplied, unlike the enhancer's params).
const SLIDERS: ParamSpec[] = [
  { key: "rotate_pitch", label: t("Rotate pitch"), kind: "range", min: -20, max: 20, default: 0 },
  { key: "rotate_yaw", label: t("Rotate yaw"), kind: "range", min: -20, max: 20, default: 0 },
  { key: "rotate_roll", label: t("Rotate roll"), kind: "range", min: -20, max: 20, default: 0 },
  { key: "blink", label: t("Blink"), kind: "range", min: -20, max: 5, default: 0 },
  { key: "eyebrow", label: t("Eyebrow"), kind: "range", min: -10, max: 15, default: 0 },
  { key: "wink", label: t("Wink"), kind: "range", min: 0, max: 25, default: 0 },
  { key: "pupil_x", label: t("Pupil X"), kind: "range", min: -15, max: 15, default: 0 },
  { key: "pupil_y", label: t("Pupil Y"), kind: "range", min: -15, max: 15, default: 0 },
  { key: "aaa", label: t("Mouth open"), kind: "range", min: -30, max: 120, default: 0 },
  { key: "eee", label: t("Mouth wide"), kind: "range", min: -20, max: 15, default: 0 },
  { key: "woo", label: t("Lips rounded"), kind: "range", min: -20, max: 15, default: 0 },
  { key: "smile", label: t("Smile"), kind: "range", min: -0.3, max: 1.3, step: 0.1, default: 0 },
  { key: "src_ratio", label: t("Source ratio"), kind: "range", min: 0, max: 1, step: 0.05, default: 1 },
  { key: "crop_factor", label: t("Crop factor"), kind: "range", min: 1.5, max: 2.5, step: 0.05, default: 1.7 },
];

const initialValues = () => {
  const v: Record<string, number> = {};
  SLIDERS.forEach((s) => (v[s.key] = s.default ?? 0));
  return v;
};

const ExpressionPanel: React.FC<{
  available: boolean;
  busy: boolean;
  onEdit: (image: string, values: Record<string, number>) => Promise<BlobResult>;
}> = ({ available, busy, onEdit }) => {
  const [values, setValues] = useState<Record<string, number>>(initialValues());
  const [image, setImage] = useState<string | null>(null);
  const [result, setResult] = useState<BlobResult | null>(null);
  const [error, setError] = useState("");

  if (!available) {
    return (
      <div className="card mb-3">
        <div className="card-body text-muted">{t("LivePortrait weights are not installed on this server.")}</div>
      </div>
    );
  }

  const run = async () => {
    if (!image) return;
    setError("");
    try {
      setResult(await onEdit(image, values));
    } catch (e: any) {
      setError(e?.message || String(e));
    }
  };

  return (
    <>
      <div className="card mb-3">
        <div className="card-body">
          <div className="text-muted small text-uppercase mb-2">{t("Expression")}</div>
          <ImageDrop label={t("Face image")} disabled={busy} onChange={setImage} />
          <div className="row mt-3">
            {SLIDERS.map((s) => (
              <RangeControl key={s.key} spec={s} value={values[s.key]} disabled={busy} onChange={(v) => setValues((val) => ({ ...val, [s.key]: v }))} />
            ))}
          </div>
          <button type="button" className="btn btn-primary" disabled={busy || !image} onClick={run}>
            {busy ? t("Editing…") : t("Apply")}
          </button>
          {error && <div className="text-danger small mt-2">{error}</div>}
        </div>
      </div>

      {result && (
        <div className="card mb-3">
          <div className="card-body">
            <img src={result.url} alt="" style={{ maxWidth: "100%", borderRadius: 6 }} />
            <div className="text-muted small mt-1">
              {result.headers["X-Face-Found"] && `${t("Face found")}: ${result.headers["X-Face-Found"]} · `}
              {result.headers["X-Edit-Ms"] && `${Number(result.headers["X-Edit-Ms"]).toFixed(0)} ms`}
            </div>
          </div>
        </div>
      )}
    </>
  );
};

export default ExpressionPanel;
