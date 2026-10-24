package videotool

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// sourceInfo is the subset of yt-dlp's metadata this page needs before committing to a full download.
type sourceInfo struct {
	Duration float64
	Title    string
}

// probeSource fetches metadata only (no download) so the duration cap can be enforced before downloading anything.
func probeSource(ctx context.Context, url string) (*sourceInfo, error) {
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--no-warnings", "--skip-download",
		"--print", "%(duration)s|%(title)s",
		url,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not read video info (unsupported link?): %w", err)
	}
	line := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	parts := strings.SplitN(line, "|", 2)
	info := &sourceInfo{}
	if len(parts) > 0 {
		info.Duration, _ = strconv.ParseFloat(parts[0], 64)
	}
	if len(parts) > 1 {
		info.Title = parts[1]
	}
	return info, nil
}

var downloadProgressRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%`)

// downloadSource runs yt-dlp, writing the best available source into destDir, reporting 0-100 download progress
// parsed from its own --newline progress output.
func downloadSource(ctx context.Context, url, destDir string, onProgress func(pct float64)) (string, error) {
	outTemplate := filepath.Join(destDir, "source.%(ext)s")
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--no-warnings",
		"--newline",
		"--max-filesize", maxFilesize,
		"-f", "bv*+ba/b",
		"-o", outTemplate,
		url,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = cmd.Stdout // --newline progress lines are written to stdout; merge stderr just in case
	if err := cmd.Start(); err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if m := downloadProgressRe.FindStringSubmatch(scanner.Text()); m != nil {
			if pct, err := strconv.ParseFloat(m[1], 64); err == nil && onProgress != nil {
				onProgress(pct)
			}
		}
	}
	scanErr := scanner.Err()
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w", err)
	}
	if scanErr != nil {
		return "", fmt.Errorf("reading yt-dlp output: %w", scanErr)
	}
	return findDownloadedFile(destDir)
}

func findDownloadedFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "source.") {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("downloaded file not found")
}
