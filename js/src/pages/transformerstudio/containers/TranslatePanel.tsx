import React, { useState } from "react";
import { t } from "@engine/controllers/i18n";

interface TranslateResult {
  predicted_text: string;
  input_tokens: number | null;
  generated_token_count: number;
  latency_ms: number;
  tokens_per_sec: number;
  temperature: number;
}

const TranslatePanel: React.FC<{ busy: boolean; onTranslate: (text: string) => Promise<TranslateResult> }> = ({
  busy,
  onTranslate,
}) => {
  const [text, setText] = useState("");
  const [result, setResult] = useState<TranslateResult | null>(null);
  const [error, setError] = useState("");

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!text.trim()) return;
    setError("");
    try {
      setResult(await onTranslate(text));
    } catch (e: any) {
      setError(e?.message || String(e));
    }
  };

  return (
    <div className="card mb-3">
      <div className="card-body">
        <div className="text-muted small text-uppercase mb-1">{t("Translate (English → Russian)")}</div>
        <form className="d-flex gap-2 mb-2" onSubmit={submit}>
          <input type="text" className="form-control" disabled={busy} value={text} onChange={(e) => setText(e.target.value)} />
          <button type="submit" className="btn btn-primary" disabled={busy || !text.trim()}>
            {t("Translate")}
          </button>
        </form>
        {error && <div className="text-danger small">{error}</div>}
        {result && (
          <>
            <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>{result.predicted_text}</pre>
            <div className="text-muted small mt-1">
              {result.input_tokens !== null && `${result.input_tokens} → `}
              {result.generated_token_count} {t("tokens")} · {result.tokens_per_sec.toFixed(1)} tok/s ·{" "}
              {result.latency_ms.toFixed(0)} ms · T={result.temperature}
            </div>
          </>
        )}
      </div>
    </div>
  );
};

export default TranslatePanel;
