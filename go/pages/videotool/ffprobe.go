package videotool

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ffprobeDuration reads a local media file's duration directly (used for uploads, which — unlike a yt-dlp link —
// have no metadata probe step of their own).
func ffprobeDuration(ctx context.Context, path string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("could not read duration: %w", err)
	}
	return d, nil
}
