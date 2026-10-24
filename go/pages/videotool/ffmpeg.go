package videotool

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// editParams is one export's requested output format + edits. Zero-ish values (Speed/Volume 1, Pitch 0) mean
// "unchanged" — see buildExportFilters. Start/End are always real timestamps (the frontend sends the full source
// duration as End when the user hasn't trimmed) — a no-op trim spanning the whole source is harmless.
type editParams struct {
	Speed  float64
	Pitch  float64 // semitones, +/-
	Volume float64
	Start  float64 // cut start, seconds
	End    float64 // cut end, seconds
	Format string

	AudioTrackPath string  // "" = no extra track
	AudioMode      string  // "replace" | "mix"
	AudioStart     float64 // seconds into the (already-cut) output timeline where the track begins
	AudioSpeed     float64
	AudioPitch     float64
	AudioVolume    float64
}

// atempoChain decomposes an arbitrary positive tempo factor into a chain of ffmpeg atempo filters, each within the
// [0.5, 2.0] range atempo itself accepts.
func atempoChain(factor float64) []string {
	if factor <= 0 {
		factor = 1
	}
	var parts []string
	f := factor
	for f > 2.0 {
		parts = append(parts, "atempo=2.0")
		f /= 2.0
	}
	for f < 0.5 {
		parts = append(parts, "atempo=0.5")
		f /= 0.5
	}
	parts = append(parts, fmt.Sprintf("atempo=%.6f", f))
	return parts
}

const ffmpegSampleRate = 44100

// buildExportFilters assembles the -filter_complex graph plus the -map targets for video and audio. sourceDuration
// resolves "no explicit end" (End <= Start) to "through the end of the source". audioOnly skips the video chain
// entirely (for mp3/m4a/wav/opus output).
func buildExportFilters(p editParams, sourceDuration float64, audioOnly bool) (filterComplex, videoMap, audioMap string) {
	cutStart := p.Start
	cutEnd := p.End
	if cutEnd <= cutStart {
		cutEnd = sourceDuration
	}
	hasCut := cutStart > 0 || cutEnd < sourceDuration

	var sb strings.Builder

	if !audioOnly {
		var vf []string
		if hasCut {
			vf = append(vf, fmt.Sprintf("trim=start=%.3f:end=%.3f", cutStart, cutEnd), "setpts=PTS-STARTPTS")
		}
		if p.Speed != 1 {
			vf = append(vf, fmt.Sprintf("setpts=PTS/%.6f", p.Speed))
		}
		if len(vf) > 0 {
			fmt.Fprintf(&sb, "[0:v]%s[vout];", strings.Join(vf, ","))
			videoMap = "[vout]"
		} else {
			videoMap = "0:v"
		}
	}

	// Original audio: same cut window, then pitch/speed/volume — pitch is shifted via asetrate+aresample (which also
	// changes tempo as a side effect), so the atempo chain corrects for that on top of whatever net speed was
	// actually requested.
	var af []string
	if hasCut {
		af = append(af, fmt.Sprintf("atrim=start=%.3f:end=%.3f", cutStart, cutEnd), "asetpts=PTS-STARTPTS")
	}
	pitchRatio := math.Pow(2, p.Pitch/12)
	if p.Pitch != 0 {
		af = append(af, fmt.Sprintf("asetrate=%.0f", ffmpegSampleRate*pitchRatio), fmt.Sprintf("aresample=%d", ffmpegSampleRate))
	}
	tempoOwed := p.Speed
	if p.Pitch != 0 {
		tempoOwed = p.Speed / pitchRatio
	}
	if tempoOwed != 1 {
		af = append(af, atempoChain(tempoOwed)...)
	}
	if p.Volume != 1 {
		af = append(af, fmt.Sprintf("volume=%.6f", p.Volume))
	}
	af = append(af, "aformat=sample_rates=44100:channel_layouts=stereo")

	if p.AudioTrackPath == "" {
		fmt.Fprintf(&sb, "[0:a]%s[aout];", strings.Join(af, ","))
		audioMap = "[aout]"
		return strings.TrimSuffix(sb.String(), ";"), videoMap, audioMap
	}

	// The extra track's own speed/pitch/volume, then positioned on the timeline via adelay — same asetrate+atempo
	// trick as the main audio chain above, generalized to the track's own independent AudioSpeed.
	trackRatio := math.Pow(2, p.AudioPitch/12)
	var tf []string
	if p.AudioPitch != 0 {
		tf = append(tf, fmt.Sprintf("asetrate=%.0f", ffmpegSampleRate*trackRatio), fmt.Sprintf("aresample=%d", ffmpegSampleRate))
	}
	trackTempoOwed := p.AudioSpeed
	if p.AudioPitch != 0 {
		trackTempoOwed = p.AudioSpeed / trackRatio
	}
	if trackTempoOwed != 1 {
		tf = append(tf, atempoChain(trackTempoOwed)...)
	}
	if p.AudioVolume != 1 {
		tf = append(tf, fmt.Sprintf("volume=%.6f", p.AudioVolume))
	}
	if delayMs := int(p.AudioStart * 1000); delayMs > 0 {
		tf = append(tf, fmt.Sprintf("adelay=%d:all=1", delayMs))
	}
	tf = append(tf, "aformat=sample_rates=44100:channel_layouts=stereo")

	if p.AudioMode == "replace" {
		fmt.Fprintf(&sb, "[1:a]%s[aout];", strings.Join(tf, ","))
	} else {
		fmt.Fprintf(&sb, "[0:a]%s[a0];", strings.Join(af, ","))
		fmt.Fprintf(&sb, "[1:a]%s[a1];", strings.Join(tf, ","))
		fmt.Fprintf(&sb, "[a0][a1]amix=inputs=2:duration=first:dropout_transition=0[aout];")
	}
	audioMap = "[aout]"
	return strings.TrimSuffix(sb.String(), ";"), videoMap, audioMap
}

func formatMime(format string) string {
	switch format {
	case "mp3":
		return "audio/mpeg"
	case "m4a":
		return "audio/mp4"
	case "wav":
		return "audio/wav"
	case "opus":
		return "audio/opus"
	case "webm":
		return "video/webm"
	case "mkv":
		return "video/x-matroska"
	default:
		return "video/mp4"
	}
}

// runFFmpeg transcodes srcPath (plus p.AudioTrackPath, if set) into destDir/output.<format>, applying p's edits.
// sourceDuration resolves an unset cut End (see buildExportFilters); totalDuration is the caller's estimate of the
// OUTPUT's duration (after cut and Speed), used to turn ffmpeg's out_time_ms into a 0-100 progress percentage.
func runFFmpeg(ctx context.Context, srcPath string, p editParams, destDir string, sourceDuration, totalDuration float64, onProgress func(pct float64)) (string, error) {
	audioOnly := audioOnlyFormats[p.Format]
	filterComplex, videoMap, audioMap := buildExportFilters(p, sourceDuration, audioOnly)
	dest := filepath.Join(destDir, "output."+p.Format)

	args := []string{"-y", "-i", srcPath}
	if p.AudioTrackPath != "" {
		args = append(args, "-i", p.AudioTrackPath)
	}
	args = append(args, "-filter_complex", filterComplex)
	if !audioOnly {
		args = append(args, "-map", videoMap)
	}
	args = append(args, "-map", audioMap)
	if p.Format == "mp3" {
		args = append(args, "-codec:a", "libmp3lame")
	}
	args = append(args, "-progress", "pipe:1", "-nostats", dest)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "out_time_ms=") || totalDuration <= 0 {
			continue
		}
		ms, err := strconv.ParseInt(strings.TrimPrefix(line, "out_time_ms="), 10, 64)
		if err != nil || onProgress == nil {
			continue
		}
		pct := (float64(ms) / 1_000_000) / totalDuration * 100
		if pct > 100 {
			pct = 100
		}
		onProgress(pct)
	}
	scanErr := scanner.Err()
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("ffmpeg failed: %w", err)
	}
	if scanErr != nil {
		return "", fmt.Errorf("reading ffmpeg output: %w", scanErr)
	}
	return dest, nil
}
