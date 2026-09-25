package actions

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
)

const maxOutputBytes = 256 << 10

// process is one Command-backed run; its pty output is buffered for the Actions page to poll.
type process struct {
	mu   sync.Mutex
	pty  *os.File
	cmd  *exec.Cmd
	buf  []byte
	base int // bytes already dropped from the front of buf
	done bool
}

// startProcess launches Command in a pty (so ssh/read prompts work) and records the run when it exits.
func (a *Action) startProcess() error {
	cmd := exec.Command(a.Command[0], a.Command[1:]...)
	cmd.Dir = a.Dir
	cmd.Env = append(os.Environ(), "TERM=xterm")
	f, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	p := &process{pty: f, cmd: cmd}
	a.mu.Lock()
	a.proc = p
	a.mu.Unlock()

	go func() {
		chunk := make([]byte, 4096)
		for {
			n, err := f.Read(chunk)
			if n > 0 {
				p.append(chunk[:n])
			}
			if err != nil {
				break
			}
		}
		waitErr := cmd.Wait()
		f.Close()
		p.mu.Lock()
		p.done = true
		p.mu.Unlock()
		if waitErr != nil {
			a.record("", fmt.Errorf("%v", waitErr))
			return
		}
		a.record("exit 0", nil)
	}()
	return nil
}

// append adds pty output, dropping the oldest bytes past maxOutputBytes.
func (p *process) append(b []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf = append(p.buf, b...)
	if over := len(p.buf) - maxOutputBytes; over > 0 {
		p.buf = p.buf[over:]
		p.base += over
	}
}

// Output returns the run's output from absolute offset from, the next offset to poll from, and whether it is still running.
func (a *Action) Output(from int) (text string, next int, running bool) {
	a.mu.Lock()
	p := a.proc
	a.mu.Unlock()
	if p == nil {
		return "", 0, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	start := from - p.base
	if start < 0 {
		start = 0
	}
	if start > len(p.buf) {
		start = len(p.buf)
	}
	return string(p.buf[start:]), p.base + len(p.buf), !p.done
}

// Input writes text to the running process's terminal, optionally followed by Enter.
func (a *Action) Input(text string, enter bool) error {
	a.mu.Lock()
	p := a.proc
	a.mu.Unlock()
	if p == nil {
		return errors.New("not running")
	}
	p.mu.Lock()
	done := p.done
	p.mu.Unlock()
	if done {
		return errors.New("not running")
	}
	if enter {
		text += "\n"
	}
	_, err := io.WriteString(p.pty, text)
	return err
}

// Stop terminates the running process's whole process group.
func (a *Action) Stop() {
	a.mu.Lock()
	p := a.proc
	a.mu.Unlock()
	if p != nil && p.cmd.Process != nil {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
	}
}
