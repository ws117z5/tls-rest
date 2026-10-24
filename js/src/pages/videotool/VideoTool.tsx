import React from "react";
import PageComponent from "@engine/containers/PageComponent";
import { t, subscribe } from "@engine/controllers/i18n";
import Timeline, { TrackKind } from "./containers/Timeline";
import { createPitchShiftNode, setPitchSemitones } from "./containers/pitchShift";

type LoadPhase = "idle" | "loading" | "loaded";
type ExportPhase = "idle" | "running" | "done" | "error";
type AudioMode = "replace" | "mix";

interface VideoToolState {
  // source
  url: string;
  loadPhase: LoadPhase;
  loadStatus: string;
  loadPct: number;
  loadError: string;
  jobId: string;
  duration: number;
  title: string;

  // edits
  format: string;
  speed: number;
  pitch: number;
  volume: number;
  cutStart: number;
  cutEnd: number;

  // audio track
  audioUploading: boolean;
  audioDuration: number; // 0 = none attached
  audioStart: number;
  audioMode: AudioMode;
  audioSpeed: number;
  audioPitch: number;
  audioVolume: number;
  useAudioTrack: boolean;

  // which track's params panel is shown
  selectedTrack: TrackKind;

  // export
  exportPhase: ExportPhase;
  exportStatus: string;
  exportPct: number;
  exportError: string;
  resultFilename: string;
}

const FORMATS = ["mp4", "webm", "mkv", "mp3", "m4a", "wav", "opus"];

export default class VideoTool extends PageComponent<{}, VideoToolState> {
  protected isPage = true;
  protected href = "video-tool";
  protected title = "Video Tool";
  protected submenu = "tools";
  protected icon = "tools";
  protected requiresAuth = true;

  private loadSource: EventSource | null = null;
  private exportSource: EventSource | null = null;
  private videoFileInput = React.createRef<HTMLInputElement>();
  private audioFileInput = React.createRef<HTMLInputElement>();
  private videoPreviewRef = React.createRef<HTMLVideoElement>();
  private audioCtx: AudioContext | null = null;
  private audioSourceNode: MediaElementAudioSourceNode | null = null;
  private pitchNode: AudioWorkletNode | null = null;

  constructor(props: {}) {
    super(props);
    this.state = {
      url: "",
      loadPhase: "idle",
      loadStatus: "",
      loadPct: 0,
      loadError: "",
      jobId: "",
      duration: 0,
      title: "",

      format: "mp4",
      speed: 1,
      pitch: 0,
      volume: 1,
      cutStart: 0,
      cutEnd: 0,

      audioUploading: false,
      audioDuration: 0,
      audioStart: 0,
      audioMode: "mix",
      audioSpeed: 1,
      audioPitch: 0,
      audioVolume: 1,
      useAudioTrack: false,

      selectedTrack: "video",

      exportPhase: "idle",
      exportStatus: "",
      exportPct: 0,
      exportError: "",
      resultFilename: "",
    };
  }

  private unsubscribeI18n?: () => void;
  async componentDidMount() {
    await super.componentDidMount();
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }
  componentWillUnmount() {
    this.unsubscribeI18n?.();
    this.loadSource?.close();
    this.exportSource?.close();
    this.audioCtx?.close();
  }

  // Speed and volume map onto real <video> properties, so the preview matches what Export will produce — Pitch runs
  // through the from-scratch AudioWorklet shifter (see pitchShift.ts), wired up once playback starts (ensureAudioGraph).
  componentDidUpdate(_prevProps: {}, prevState: VideoToolState) {
    const el = this.videoPreviewRef.current;
    if (el) {
      if (prevState.volume !== this.state.volume) {
        el.volume = Math.min(1, Math.max(0, this.state.volume));
      }
      if (prevState.speed !== this.state.speed) {
        el.playbackRate = this.state.speed;
      }
    }
    if (prevState.pitch !== this.state.pitch) {
      if (this.pitchNode) setPitchSemitones(this.pitchNode, this.state.pitch);
      // Only re-route when crossing the "shift active" boundary (0 <-> non-0), not on every slider tick within it —
      // disconnect/connect itself produces an audible click, so it isn't something to do on every change.
      if ((prevState.pitch === 0) !== (this.state.pitch === 0)) {
        this.applyAudioRouting();
      }
    }
  }

  private applyLivePreview = (el: HTMLVideoElement) => {
    el.volume = Math.min(1, Math.max(0, this.state.volume));
    el.playbackRate = this.state.speed;
  };

  private seekTo = (t: number) => {
    const el = this.videoPreviewRef.current;
    if (el) el.currentTime = t;
  };

  // Lazily builds the <video> -> (pitch-shift, only when needed) -> destination Web Audio graph the first time
  // playback starts (a real user gesture, required by browser autoplay policy to create/resume an AudioContext).
  // createMediaElementSource can only be called once per element, so this is a no-op on a second play.
  private ensureAudioGraph = async (el: HTMLVideoElement) => {
    if (this.audioCtx) {
      await this.audioCtx.resume();
      return;
    }
    const AudioContextCtor = window.AudioContext || (window as any).webkitAudioContext;
    const ctx: AudioContext = new AudioContextCtor();
    this.audioCtx = ctx;
    // createMediaElementSource immediately redirects the element's audio output into the Web Audio graph — from
    // this point on, if nothing downstream ever reaches ctx.destination, playback goes silent. applyAudioRouting
    // always connects *something* to destination, even if the worklet never loads, so this can't leave audio
    // silently orphaned.
    this.audioSourceNode = ctx.createMediaElementSource(el);
    try {
      const pitchNode = await createPitchShiftNode(ctx);
      this.pitchNode = pitchNode;
      setPitchSemitones(pitchNode, this.state.pitch);
      pitchNode.connect(ctx.destination);
    } catch (err) {
      console.warn("Pitch preview unavailable, playing unshifted audio only:", err);
      this.pitchNode = null;
    }
    this.applyAudioRouting();
  };

  // Routes the live source straight to the speakers whenever Pitch is 0 — the granular shifter isn't a perfect
  // passthrough even at a 1:1 ratio (it blends two half-page-offset reads of the same buffered audio), so bypassing
  // it entirely for the common "pitch untouched" case keeps default playback pristine. Only routes through the
  // worklet once the user actually moves the Pitch slider.
  private applyAudioRouting = () => {
    const { audioSourceNode, pitchNode, audioCtx } = this;
    if (!audioSourceNode || !audioCtx) return;
    audioSourceNode.disconnect();
    if (this.state.pitch !== 0 && pitchNode) {
      audioSourceNode.connect(pitchNode);
    } else {
      audioSourceNode.connect(audioCtx.destination);
    }
  };

  private resetForNewSource() {
    this.loadSource?.close();
    this.exportSource?.close();
    this.audioCtx?.close();
    this.audioCtx = null;
    this.audioSourceNode = null;
    this.pitchNode = null;
    this.setState({
      jobId: "", duration: 0, title: "",
      cutStart: 0, cutEnd: 0,
      audioDuration: 0, audioStart: 0, useAudioTrack: false, audioSpeed: 1, audioPitch: 0, audioVolume: 1,
      selectedTrack: "video",
      exportPhase: "idle", exportStatus: "", exportPct: 0, exportError: "", resultFilename: "",
    });
  }

  private loadFromUrl = () => {
    const { url } = this.state;
    if (!url.trim()) return;
    this.resetForNewSource();
    this.setState({ loadPhase: "loading", loadStatus: t("Starting…"), loadPct: 0, loadError: "" });

    const es = new EventSource(`/api/video-tool/load?url=${encodeURIComponent(url.trim())}`);
    this.loadSource = es;

    es.addEventListener("status", (e: MessageEvent) => {
      this.setState({ loadStatus: JSON.parse(e.data).message });
    });
    es.addEventListener("download_progress", (e: MessageEvent) => {
      this.setState({ loadPct: JSON.parse(e.data).percent });
    });
    es.addEventListener("complete", (e: MessageEvent) => {
      const data = JSON.parse(e.data);
      this.setState({
        loadPhase: "loaded", jobId: data.id, duration: data.duration, title: data.title,
        cutStart: 0, cutEnd: data.duration,
      });
      es.close();
    });
    es.addEventListener("error", (e: MessageEvent) => {
      const message = (e as MessageEvent).data ? JSON.parse((e as MessageEvent).data).message : t("Connection lost.");
      this.setState({ loadPhase: "idle", loadError: message });
      es.close();
    });
  };

  private pickVideoFile = () => this.videoFileInput.current?.click();
  private pickAudioFile = () => this.audioFileInput.current?.click();

  private uploadVideoFile = async (file: File) => {
    this.resetForNewSource();
    this.setState({ loadPhase: "loading", loadStatus: t("Uploading…"), loadPct: 0, loadError: "" });
    try {
      const body = new FormData();
      body.append("file", file);
      const res = await fetch("/api/video-tool/upload", { method: "POST", body });
      if (!res.ok) throw new Error((await res.json().catch(() => null))?.error || res.statusText);
      const data = await res.json();
      this.setState({
        loadPhase: "loaded", jobId: data.id, duration: data.duration, title: data.title,
        cutStart: 0, cutEnd: data.duration,
      });
    } catch (err: any) {
      this.setState({ loadPhase: "idle", loadError: err?.message || t("Upload failed.") });
    }
  };

  private uploadAudioFile = async (file: File) => {
    const { jobId } = this.state;
    if (!jobId) return;
    this.setState({ audioUploading: true });
    try {
      const body = new FormData();
      body.append("file", file);
      const res = await fetch(`/api/video-tool/upload-audio?job=${encodeURIComponent(jobId)}`, { method: "POST", body });
      if (!res.ok) throw new Error((await res.json().catch(() => null))?.error || res.statusText);
      const data = await res.json();
      this.setState({
        audioUploading: false, audioDuration: data.duration, audioStart: 0, useAudioTrack: true,
        selectedTrack: "audio",
      });
    } catch (err: any) {
      this.setState({ audioUploading: false, loadError: err?.message || t("Audio upload failed.") });
    }
  };

  private startExport = () => {
    const {
      jobId, format, speed, pitch, volume, cutStart, cutEnd,
      useAudioTrack, audioMode, audioStart, audioSpeed, audioPitch, audioVolume,
    } = this.state;
    if (!jobId) return;

    this.exportSource?.close();
    this.setState({ exportPhase: "running", exportStatus: t("Starting…"), exportPct: 0, exportError: "", resultFilename: "" });

    const params = new URLSearchParams({
      format, speed: String(speed), pitch: String(pitch), volume: String(volume),
      start: String(cutStart), end: String(cutEnd),
    });
    if (useAudioTrack) {
      params.set("useAudioTrack", "1");
      params.set("audioMode", audioMode);
      params.set("audioStart", String(audioStart));
      params.set("audioSpeed", String(audioSpeed));
      params.set("audioPitch", String(audioPitch));
      params.set("audioVolume", String(audioVolume));
    }

    const es = new EventSource(`/api/video-tool/export/${jobId}/events?${params.toString()}`);
    this.exportSource = es;

    es.addEventListener("status", (e: MessageEvent) => {
      this.setState({ exportStatus: JSON.parse(e.data).message });
    });
    es.addEventListener("edit_progress", (e: MessageEvent) => {
      this.setState({ exportPct: JSON.parse(e.data).percent });
    });
    es.addEventListener("complete", (e: MessageEvent) => {
      const data = JSON.parse(e.data);
      this.setState({ exportPhase: "done", resultFilename: data.filename });
      es.close();
    });
    es.addEventListener("error", (e: MessageEvent) => {
      const message = (e as MessageEvent).data ? JSON.parse((e as MessageEvent).data).message : t("Connection lost.");
      this.setState({ exportPhase: "error", exportError: message });
      es.close();
    });
  };

  render() {
    const {
      url, loadPhase, loadStatus, loadPct, loadError, jobId, duration, title,
      format, speed, pitch, volume, cutStart, cutEnd,
      audioUploading, audioDuration, audioStart, audioMode, audioSpeed, audioPitch, audioVolume, useAudioTrack,
      selectedTrack,
      exportPhase, exportStatus, exportPct, exportError, resultFilename,
    } = this.state;
    const loading = loadPhase === "loading";
    const loaded = loadPhase === "loaded";
    const exporting = exportPhase === "running";

    return (
      <div className="container-fluid pt-4">
        <h4 className="mb-3">{t("Video Tool")}</h4>

        <div className="card mb-3">
          <div className="card-body">
            <div className="text-muted small text-uppercase mb-1">{t("Link")}</div>
            <div className="d-flex gap-2 mb-2">
              <input
                className="form-control"
                value={url}
                disabled={loading}
                onChange={(e) => this.setState({ url: e.target.value })}
                placeholder={t("YouTube / Instagram / TikTok link…")}
                spellCheck={false}
              />
              <button className="btn btn-primary text-nowrap" disabled={loading || !url.trim()} onClick={this.loadFromUrl}>
                {t("Load")}
              </button>
              <button className="btn btn-outline-secondary text-nowrap" disabled={loading} onClick={this.pickVideoFile}>
                {t("Upload from drive")}
              </button>
              <input
                ref={this.videoFileInput}
                type="file"
                accept="video/*"
                style={{ display: "none" }}
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  e.target.value = "";
                  if (file) this.uploadVideoFile(file);
                }}
              />
            </div>

            {loading && (
              <div className="mt-2">
                <div className="small text-muted mb-1">{loadStatus}</div>
                <div className="progress" style={{ height: 6 }}>
                  <div className="progress-bar" style={{ width: `${loadPct}%` }} />
                </div>
              </div>
            )}
            {loadError && <div className="text-danger small mt-2">{loadError}</div>}
          </div>
        </div>

        {loaded && (
          <div className="card mb-3">
            <div className="card-body">
              {title && <div className="text-muted small mb-2">{title}</div>}
              {/* Native volume/mute and the overflow ("…") menu are hidden — the Volume slider below is the single
                  source of truth, and a native 0-100% control would just invite a second, conflicting "volume". */}
              <style>{`
                .video-tool-preview::-webkit-media-controls-volume-slider,
                .video-tool-preview::-webkit-media-controls-mute-button,
                .video-tool-preview::-webkit-media-controls-overflow-button {
                  display: none !important;
                }
              `}</style>
              <video
                key={jobId}
                ref={this.videoPreviewRef}
                className="video-tool-preview"
                controls
                controlsList="nodownload noremoteplayback"
                disablePictureInPicture
                disableRemotePlayback
                onLoadedMetadata={(e) => this.applyLivePreview(e.currentTarget)}
                onPlay={(e) => this.ensureAudioGraph(e.currentTarget)}
                src={`/api/video-tool/source/${jobId}`}
                style={{ width: "100%", maxHeight: 420, background: "#000" }}
              />

              <Timeline
                duration={duration}
                cutStart={cutStart}
                cutEnd={cutEnd}
                onCutChange={(s, en) => this.setState({ cutStart: s, cutEnd: en })}
                audioDuration={audioDuration || undefined}
                audioStart={audioStart}
                onAudioStartChange={(s) => this.setState({ audioStart: s })}
                selectedTrack={selectedTrack}
                onSelectTrack={(t2) => this.setState({ selectedTrack: t2 })}
                onSeek={this.seekTo}
              />

              <div className="mt-3">
                <button className="btn btn-outline-secondary btn-sm" disabled={audioUploading} onClick={this.pickAudioFile}>
                  {audioUploading ? t("Uploading…") : audioDuration ? t("Replace audio track file") : t("Add audio track")}
                </button>
                <input
                  ref={this.audioFileInput}
                  type="file"
                  accept="audio/*"
                  style={{ display: "none" }}
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    e.target.value = "";
                    if (file) this.uploadAudioFile(file);
                  }}
                />
              </div>

              {/* Track params — click a row in the timeline above to switch which track's controls show here. */}
              {selectedTrack === "video" ? (
                <div className="row mt-3">
                  <div className="col-12 mb-1">
                    <div className="text-muted small text-uppercase">{t("Video track")}</div>
                  </div>
                  <div className="col-6 col-md-4 mb-3">
                    <div className="text-muted small text-uppercase mb-1">{t("Speed")}: {speed}x</div>
                    <input
                      type="range" className="form-control"
                      min={0.25} max={4} step={0.05} value={speed}
                      onChange={(e) => this.setState({ speed: Number(e.target.value) })}
                    />
                  </div>
                  <div className="col-6 col-md-4 mb-3">
                    <div className="text-muted small text-uppercase mb-1">{t("Pitch")}: {pitch} <span title={t("Live preview — the exported file may sound slightly different, since Export uses a separate, higher-quality shift.")}>ⓘ</span></div>
                    <input
                      type="range" className="form-control"
                      min={-12} max={12} step={1} value={pitch}
                      onChange={(e) => this.setState({ pitch: Number(e.target.value) })}
                    />
                  </div>
                  <div className="col-6 col-md-4 mb-3">
                    <div className="text-muted small text-uppercase mb-1">{t("Volume")}: {volume}x</div>
                    <input
                      type="range" className="form-control"
                      min={0} max={5} step={0.1} value={volume}
                      onChange={(e) => this.setState({ volume: Number(e.target.value) })}
                    />
                  </div>
                </div>
              ) : (
                <div className="row mt-3">
                  <div className="col-12 mb-1">
                    <div className="text-muted small text-uppercase">{t("Audio track")} <span title={t("Speed/pitch/volume here preview on Export only — the live preview still plays the video's own original audio.")}>ⓘ</span></div>
                  </div>
                  <div className="col-6 col-md-4 mb-2">
                    <div className="text-muted small text-uppercase mb-1">{t("Mode")}</div>
                    <select
                      className="form-control"
                      value={audioMode}
                      onChange={(e) => this.setState({ audioMode: e.target.value as AudioMode })}
                    >
                      <option value="mix">{t("Mix with original")}</option>
                      <option value="replace">{t("Replace original")}</option>
                    </select>
                  </div>
                  <div className="col-6 col-md-4 mb-2">
                    <div className="text-muted small text-uppercase mb-1">{t("Speed")}: {audioSpeed}x</div>
                    <input
                      type="range" className="form-control"
                      min={0.25} max={4} step={0.05} value={audioSpeed}
                      onChange={(e) => this.setState({ audioSpeed: Number(e.target.value) })}
                    />
                  </div>
                  <div className="col-6 col-md-4 mb-2">
                    <div className="text-muted small text-uppercase mb-1">{t("Pitch")}: {audioPitch}</div>
                    <input
                      type="range" className="form-control"
                      min={-12} max={12} step={1} value={audioPitch}
                      onChange={(e) => this.setState({ audioPitch: Number(e.target.value) })}
                    />
                  </div>
                  <div className="col-6 col-md-4 mb-2">
                    <div className="text-muted small text-uppercase mb-1">{t("Volume")}: {audioVolume}x</div>
                    <input
                      type="range" className="form-control"
                      min={0} max={5} step={0.1} value={audioVolume}
                      onChange={(e) => this.setState({ audioVolume: Number(e.target.value) })}
                    />
                  </div>
                  <div className="col-6 col-md-4 mb-2 d-flex align-items-end">
                    <div className="form-check">
                      <input
                        className="form-check-input" type="checkbox" id="useAudioTrack"
                        checked={useAudioTrack}
                        onChange={(e) => this.setState({ useAudioTrack: e.target.checked })}
                      />
                      <label className="form-check-label small" htmlFor="useAudioTrack">{t("Use this track on export")}</label>
                    </div>
                  </div>
                </div>
              )}

              <div className="mb-3" style={{ maxWidth: 200 }}>
                <div className="text-muted small text-uppercase mb-1">{t("Format")}</div>
                <select className="form-control" value={format} onChange={(e) => this.setState({ format: e.target.value })}>
                  {FORMATS.map((f) => <option key={f} value={f}>{f}</option>)}
                </select>
              </div>

              <button className="btn btn-primary" disabled={exporting} onClick={this.startExport}>
                {exporting ? t("Exporting…") : t("Export")}
              </button>

              {exportPhase !== "idle" && (
                <div className="mt-3">
                  <div className="text-muted small mb-2">{exportStatus}</div>
                  {exporting && (
                    <div className="progress mb-2" style={{ height: 6 }}>
                      <div className="progress-bar" style={{ width: `${exportPct}%` }} />
                    </div>
                  )}
                  {exportPhase === "done" && (
                    <a className="btn btn-success" href={`/api/video-tool/download/${jobId}`} download={resultFilename}>
                      {t("Download")} {resultFilename}
                    </a>
                  )}
                  {exportPhase === "error" && <div className="text-danger small">{exportError}</div>}
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    );
  }
}
