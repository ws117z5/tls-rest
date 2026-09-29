import React, { useCallback, useEffect, useRef, useState } from "react";
import PageComponent from "@engine/containers/PageComponent";
import { t, subscribe } from "@engine/controllers/i18n";
import ServerBar from "./containers/ServerBar";
import ModelsPanel, { ModelEntry } from "./containers/ModelsPanel";
import TrainPanel from "./containers/TrainPanel";
import TranslatePanel from "./containers/TranslatePanel";
import ImagesPanel from "./containers/ImagesPanel";
import EnhancePanel from "./containers/EnhancePanel";
import ExpressionPanel from "./containers/ExpressionPanel";
import VideoToolPanel from "./containers/VideoToolPanel";
import { ParamSpec } from "./containers/RangeControl";
import * as api from "./containers/api";

const STORAGE_KEY = "transformerStudio.server";

type Tab = "models" | "train" | "translate" | "images" | "enhance" | "expression" | "video";

const TABS_FOR_KIND: Record<string, Tab[]> = {
  native: ["models", "train", "translate"],
  marian: ["models", "translate"],
  diffusion: ["models", "images"],
  enhancer: ["models", "enhance"],
};

const TAB_LABEL: Record<Tab, string> = {
  models: "Models",
  train: "Train",
  translate: "Translate",
  images: "Text to image",
  enhance: "Enhance",
  expression: "Face expression",
  video: "Image to video",
};

const StudioApp: React.FC = () => {
  const [, forceUpdate] = useState(0);
  useEffect(() => subscribe(() => forceUpdate((n) => n + 1)), []);

  const [server, setServer] = useState(() => {
    try {
      return localStorage.getItem(STORAGE_KEY) || "";
    } catch {
      return "";
    }
  });
  const [connecting, setConnecting] = useState(false);
  const [connected, setConnected] = useState(false);
  const [connectError, setConnectError] = useState("");
  const [status, setStatus] = useState<Record<string, any>>({});
  const [models, setModels] = useState<ModelEntry[]>([]);
  const [tab, setTab] = useState<Tab>("models");

  const [lossHistory, setLossHistory] = useState<number[]>([]);
  const [progress, setProgress] = useState(0);
  const [message, setMessage] = useState("");
  const [expressionAvailable, setExpressionAvailable] = useState(false);
  const [videoAvailable, setVideoAvailable] = useState(false);
  const [videoInfo, setVideoInfo] = useState<any>(null);
  const [enhancerParams, setEnhancerParams] = useState<ParamSpec[]>([]);

  const [busy, setBusy] = useState("");
  const [actionError, setActionError] = useState("");

  const unsubscribeEvents = useRef<(() => void) | null>(null);

  const refreshModels = useCallback(async (srv: string) => {
    const data = await api.getModels(srv);
    const list: ModelEntry[] = data?.models || [];
    setModels(list);
    const selected = list.find((m) => m.selected);
    if (selected?.params) setEnhancerParams(selected.params);
  }, []);

  const connect = async () => {
    const base = server.trim();
    if (!base) return;
    setConnecting(true);
    setConnectError("");
    try {
      const s = await api.getStatus(base);
      setStatus(s || {});
      setLossHistory(Array.isArray(s?.loss_history) ? s.loss_history : []);
      setConnected(true);
      try {
        localStorage.setItem(STORAGE_KEY, base);
      } catch {
        // storage unavailable
      }
      await refreshModels(base);
      const kind = s?.kind;
      setTab((TABS_FOR_KIND[kind] || ["models"])[0]);

      api.getExpressionInfo(base).then((r) => setExpressionAvailable(!!r?.available)).catch(() => setExpressionAvailable(false));
      api.getVideoInfo(base).then((r) => {
        setVideoAvailable(!!r?.available);
        setVideoInfo(r?.info || null);
      }).catch(() => setVideoAvailable(false));

      unsubscribeEvents.current = api.subscribeEvents(base, (e) => {
        if (e.type === "status") {
          setStatus((prev) => ({ ...prev, ...e }));
          if (Array.isArray(e.loss_history)) setLossHistory(e.loss_history);
        } else if (e.type === "progress") {
          const p = Number(e.progress) || 0;
          setProgress(p);
          setMessage(e.message || "");
          if (typeof e.current_loss === "number") setLossHistory((h) => [...h, e.current_loss]);
        } else if (e.type === "epoch_complete") {
          setProgress(100);
          setMessage(e.message || "");
          if (typeof e.current_loss === "number") setLossHistory((h) => [...h, e.current_loss]);
        } else if (e.type === "populate_status") {
          setMessage(e.message || "");
          setStatus((prev) => ({ ...prev, is_populating: false, tokenizer: e.tokenizer ?? prev.tokenizer }));
        } else if (e.type === "generate_progress" || e.type === "video_progress") {
          setMessage(e.message || "");
        }
      });
    } catch (e: any) {
      setConnectError(`${t("Could not connect")}: ${e?.message || e}`);
      setConnected(false);
    } finally {
      setConnecting(false);
    }
  };

  const disconnect = () => {
    unsubscribeEvents.current?.();
    unsubscribeEvents.current = null;
    setConnected(false);
    setStatus({});
    setModels([]);
    setLossHistory([]);
    setProgress(0);
    setMessage("");
  };

  useEffect(() => () => unsubscribeEvents.current?.(), []);

  const run = useCallback(
    async <T,>(label: string, fn: () => Promise<T>): Promise<T> => {
      setBusy(label);
      setActionError("");
      try {
        return await fn();
      } catch (e: any) {
        setActionError(`${label}: ${e?.message || e}`);
        throw e;
      } finally {
        setBusy("");
      }
    },
    []
  );

  const kind = status.kind;
  const availableTabs = (TABS_FOR_KIND[kind] || ["models"]).concat(["expression", "video"]);

  return (
    <div className="container-fluid pt-4">
      <h4 className="mb-3">{t("Transformer Studio")}</h4>

      <ServerBar
        server={server}
        onServerChange={setServer}
        connected={connected}
        connecting={connecting}
        error={connectError}
        status={status}
        onConnect={connect}
        onDisconnect={disconnect}
      />

      {connected && (
        <>
          <ul className="nav nav-tabs mb-3">
            {availableTabs.map((tb) => (
              <li className="nav-item" key={tb}>
                <button type="button" className={`nav-link${tab === tb ? " active" : ""}`} onClick={() => setTab(tb)}>
                  {t(TAB_LABEL[tb])}
                </button>
              </li>
            ))}
          </ul>

          {actionError && <div className="alert alert-danger">{actionError}</div>}

          {tab === "models" && (
            <ModelsPanel
              models={models}
              busy={!!busy}
              onSelect={(name) =>
                run(t("Select model"), () => api.selectModel(server, name)).then(() => {
                  api.getStatus(server).then(setStatus);
                  refreshModels(server);
                })
              }
              onCreate={(params) =>
                run(t("Create model"), () => api.createModel(server, params)).then(() => {
                  api.getStatus(server).then(setStatus);
                  refreshModels(server);
                })
              }
            />
          )}

          {tab === "train" && kind === "native" && (
            <TrainPanel
              status={status}
              lossHistory={lossHistory}
              progress={progress}
              message={message}
              busy={busy}
              onBuildTokenizer={(name, targetMerges, populationSampleSize) =>
                run(t("Build tokenizer"), () =>
                  api.buildTokenizer(server, { name, target_merges: targetMerges, population_sample_size: populationSampleSize })
                ).then(() => setStatus((s) => ({ ...s, is_populating: true })))
              }
              onSaveConfig={(params) =>
                run(t("Save config"), () => api.setConfig(server, params)).then((r) => setStatus((s) => ({ ...s, ...r })))
              }
              onStart={() =>
                run(t("Start epoch"), () => api.trainStart(server)).then(() => {
                  setProgress(0);
                  setStatus((s) => ({ ...s, is_training: true }));
                })
              }
              onStop={() => run(t("Stop epoch"), () => api.trainStop(server)).then(() => setStatus((s) => ({ ...s, is_training: false })))}
            />
          )}

          {tab === "translate" && (kind === "native" || kind === "marian") && (
            <TranslatePanel busy={!!busy} onTranslate={(text) => run(t("Translate"), () => api.translate(server, text))} />
          )}

          {tab === "images" && kind === "diffusion" && (
            <ImagesPanel
              defaults={status.image_defaults || {}}
              busy={!!busy}
              onGenerate={(params) => run(t("Generate"), () => api.generateImage(server, params))}
            />
          )}

          {tab === "enhance" && kind === "enhancer" && (
            <EnhancePanel
              params={enhancerParams}
              busyImage={busy === t("Enhance")}
              busyVideo={busy === t("Enhance video")}
              onEnhanceImage={(image, values) => run(t("Enhance"), () => api.enhanceImage(server, image, values))}
              onEnhanceVideo={(file, values, maxFrames, varyNoise) =>
                run(t("Enhance video"), () => api.enhanceVideo(server, file, values, maxFrames, varyNoise))
              }
            />
          )}

          {tab === "expression" && (
            <ExpressionPanel
              available={expressionAvailable}
              busy={!!busy}
              onEdit={(image, values) => run(t("Apply"), () => api.editExpression(server, image, values))}
            />
          )}

          {tab === "video" && (
            <VideoToolPanel
              available={videoAvailable}
              info={videoInfo}
              busy={!!busy}
              onGenerate={(params) => run(t("Generate video"), () => api.generateVideo(server, params))}
            />
          )}
        </>
      )}
    </div>
  );
};

export default class TransformerStudio extends PageComponent<{}, {}> {
  protected isPage = true;
  protected href = "transformer-studio";
  protected title = "Transformer Studio";
  protected submenu = "tools";
  protected icon = "statistics";
  protected requiresAuth = true;

  render() {
    return <StudioApp />;
  }
}
