import React, { useCallback, useEffect, useRef, useState } from "react";

export type TrackKind = "video" | "audio";

export interface TimelineProps {
  duration: number; // total source duration, seconds
  cutStart: number;
  cutEnd: number;
  onCutChange: (start: number, end: number) => void;

  // Audio track row — only rendered once a track is attached.
  audioDuration?: number;
  audioStart: number;
  onAudioStartChange: (start: number) => void;

  selectedTrack: TrackKind;
  onSelectTrack: (track: TrackKind) => void;

  // Clicking any track's background (not a drag handle/block) seeks the video preview there.
  onSeek: (seconds: number) => void;
}

type Drag = "cut-start" | "cut-end" | "audio" | null;

const fmt = (s: number): string => {
  const m = Math.floor(s / 60);
  const sec = Math.floor(s % 60);
  return `${m}:${sec.toString().padStart(2, "0")}`;
};

// A video-editor-style timeline: a labeled Video track (with two draggable trim handles), a labeled Audio track
// directly below it — the clip's own original audio, shown as soon as a source is loaded, mirroring the video's cut
// range (it isn't independently trimmable: the backend always cuts video and its own audio together) — and, once
// one is attached, a further "Audio Track" row for the separately uploaded/replaced audio, which IS independently
// draggable. Clicking the Video or (original) Audio row both select the "video" track's params (Speed/Pitch/Volume
// — they're the same edit, applied to both streams together); clicking "Audio Track" selects that track's own,
// separate params.
const Timeline: React.FC<TimelineProps> = ({
  duration, cutStart, cutEnd, onCutChange,
  audioDuration, audioStart, onAudioStartChange,
  selectedTrack, onSelectTrack, onSeek,
}) => {
  const trackRef = useRef<HTMLDivElement>(null);
  const [drag, setDrag] = useState<Drag>(null);
  const audioDragOffset = useRef(0); // seconds between pointer and the block's own start, captured at drag start

  const xToSeconds = useCallback((clientX: number): number => {
    const el = trackRef.current;
    if (!el || duration <= 0) return 0;
    const rect = el.getBoundingClientRect();
    const ratio = Math.min(1, Math.max(0, (clientX - rect.left) / rect.width));
    return ratio * duration;
  }, [duration]);

  useEffect(() => {
    if (!drag) return undefined;
    const onMove = (e: MouseEvent) => {
      const t = xToSeconds(e.clientX);
      if (drag === "cut-start") {
        onCutChange(Math.min(t, cutEnd - 0.25), cutEnd);
      } else if (drag === "cut-end") {
        onCutChange(cutStart, Math.max(t, cutStart + 0.25));
      } else if (drag === "audio" && audioDuration) {
        const maxStart = Math.max(0, duration - audioDuration);
        onAudioStartChange(Math.min(maxStart, Math.max(0, t - audioDragOffset.current)));
      }
    };
    const onUp = () => setDrag(null);
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    return () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
  }, [drag, xToSeconds, cutStart, cutEnd, duration, audioDuration, onCutChange, onAudioStartChange]);

  const pct = (s: number): string => (duration > 0 ? `${(s / duration) * 100}%` : "0%");

  const rowStyle = (selected: boolean): React.CSSProperties => ({
    display: "flex", alignItems: "center", gap: 8,
    padding: 4, borderRadius: 4, cursor: "pointer",
    outline: selected ? "2px solid #4da3ff" : "2px solid transparent",
  });
  const labelStyle: React.CSSProperties = { width: 84, flexShrink: 0, fontSize: 12, textTransform: "uppercase", opacity: 0.7 };

  return (
    <div className="mt-3">
      <div className="d-flex justify-content-between small text-muted mb-1" style={{ paddingLeft: 92 }}>
        <span>0:00</span>
        <span>{fmt(duration)}</span>
      </div>

      {/* Video track — always first/above. */}
      <div style={rowStyle(selectedTrack === "video")} onMouseDown={() => onSelectTrack("video")}>
        <span style={labelStyle}>{"Video"}</span>
        <div
          ref={trackRef}
          onMouseDown={(e) => onSeek(xToSeconds(e.clientX))}
          style={{ position: "relative", height: 36, flex: 1, background: "rgba(255,255,255,0.08)", borderRadius: 4 }}
        >
          <div
            style={{
              position: "absolute", top: 0, bottom: 0,
              left: pct(cutStart), width: `calc(${pct(cutEnd)} - ${pct(cutStart)})`,
              background: "rgba(80,160,255,0.35)", borderRadius: 4,
            }}
          />
          <div
            onMouseDown={(e) => { e.stopPropagation(); onSelectTrack("video"); setDrag("cut-start"); }}
            style={{
              position: "absolute", left: pct(cutStart), top: 0, bottom: 0,
              width: 8, marginLeft: -4, cursor: "ew-resize", background: "#4da3ff", borderRadius: 2,
            }}
            title={fmt(cutStart)}
          />
          <div
            onMouseDown={(e) => { e.stopPropagation(); onSelectTrack("video"); setDrag("cut-end"); }}
            style={{
              position: "absolute", left: pct(cutEnd), top: 0, bottom: 0,
              width: 8, marginLeft: -4, cursor: "ew-resize", background: "#4da3ff", borderRadius: 2,
            }}
            title={fmt(cutEnd)}
          />
        </div>
      </div>
      <div className="small text-muted mt-1" style={{ paddingLeft: 92 }}>{fmt(cutStart)} – {fmt(cutEnd)}</div>

      {/* The clip's own original audio — shown as soon as a source is loaded, mirroring the video's cut range (not
          independently draggable: the backend always cuts video and its own audio together). Selecting it shows the
          same Video params, since that's what actually governs it. */}
      {duration > 0 && (
        <div style={{ ...rowStyle(selectedTrack === "video"), marginTop: 6 }} onMouseDown={() => onSelectTrack("video")}>
          <span style={labelStyle}>{"Audio"}</span>
          <div
            onMouseDown={(e) => onSeek(xToSeconds(e.clientX))}
            style={{ position: "relative", height: 24, flex: 1, background: "rgba(255,255,255,0.05)", borderRadius: 4 }}
          >
            <div
              style={{
                position: "absolute", top: 0, bottom: 0,
                left: pct(cutStart), width: `calc(${pct(cutEnd)} - ${pct(cutStart)})`,
                background: "rgba(80,160,255,0.2)", borderRadius: 4,
              }}
            />
          </div>
        </div>
      )}

      {/* The separately uploaded/replaced audio track — only once one is attached. Unlike the clip's own audio
          above, this one IS independently draggable (its own position on the timeline). */}
      {audioDuration ? (
        <>
          <div style={{ ...rowStyle(selectedTrack === "audio"), marginTop: 6 }} onMouseDown={() => onSelectTrack("audio")}>
            <span style={labelStyle}>{"Audio Track"}</span>
            <div
              onMouseDown={(e) => onSeek(xToSeconds(e.clientX))}
              style={{ position: "relative", height: 28, flex: 1, background: "rgba(255,255,255,0.05)", borderRadius: 4 }}
            >
              <div
                onMouseDown={(e) => {
                  e.stopPropagation();
                  onSelectTrack("audio");
                  audioDragOffset.current = xToSeconds(e.clientX) - audioStart;
                  setDrag("audio");
                }}
                style={{
                  position: "absolute", top: 0, bottom: 0,
                  left: pct(audioStart), width: pct(Math.min(audioDuration, duration - audioStart)),
                  background: "rgba(80,220,140,0.4)", borderRadius: 4, cursor: "grab",
                }}
              />
            </div>
          </div>
          <div className="small text-muted mt-1" style={{ paddingLeft: 92 }}>
            {fmt(audioStart)} – {fmt(Math.min(duration, audioStart + audioDuration))}
          </div>
        </>
      ) : null}
    </div>
  );
};

export default Timeline;
