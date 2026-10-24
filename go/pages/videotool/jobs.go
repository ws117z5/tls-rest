package videotool

import (
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// job is one edit session: a loaded/uploaded source, an optional extra audio track, and (once exported) an output
// file. Everything lives under one temp directory, all swept together after retention — see expireJob.
type job struct {
	dir string

	mu             sync.Mutex
	sourcePath     string
	sourceDuration float64
	title          string
	audioPath      string
	audioDuration  float64
	outputPath     string
	outputFilename string
	outputMime     string
}

var (
	jobsMu sync.Mutex
	jobs   = map[string]*job{}
)

// newJob allocates a fresh job directory and registers it, scheduling its removal after retention regardless of
// whether it's ever used further — so an abandoned load/upload can't linger on disk.
func newJob() (string, *job, error) {
	dir, err := os.MkdirTemp("", workDirPrefix)
	if err != nil {
		return "", nil, err
	}
	id := uuid.NewString()
	j := &job{dir: dir}
	jobsMu.Lock()
	jobs[id] = j
	jobsMu.Unlock()
	time.AfterFunc(retention, func() { expireJob(id) })
	return id, j, nil
}

func getJob(id string) (*job, bool) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	j, ok := jobs[id]
	return j, ok
}

func expireJob(id string) {
	jobsMu.Lock()
	j, ok := jobs[id]
	if ok {
		delete(jobs, id)
	}
	jobsMu.Unlock()
	if ok {
		_ = os.RemoveAll(j.dir)
	}
}
