//go:build linux

package codex

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/pelletier/go-toml/v2"
)

// Server manages one codex app-server process. A stopped server may be restarted.
type Server struct {
	user     string
	workdir  string
	mu       sync.Mutex
	cmd      *exec.Cmd
	done     chan struct{}
	result   *serverResult
	stopping bool
}

type serverResult struct{ err error }

// Start launches the server. Success indicates process creation, not socket readiness.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil {
		return errors.New("codex server is already running or stopping")
	}

	cmd := exec.Command(
		"codex",
		"app-server",
		"--listen",
		fmt.Sprintf("unix:///run/codex/%s.sock", s.user),
	)
	cmd.Dir = s.workdir
	cmd.Env = append(os.Environ(), "CODEX_HOME="+filepath.Join(s.workdir, ".codex"))
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start codex server: %w", err)
	}

	s.cmd, s.done, s.result = cmd, make(chan struct{}), &serverResult{}
	done, result := s.done, s.result
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		defer s.mu.Unlock()
		result.err = err
		if !s.stopping {
			s.cmd = nil
		}
		close(done)
	}()
	return nil
}

// Wait waits for the current (or most recent) process and returns its exit error.
func (s *Server) Wait() error {
	s.mu.Lock()
	done, result := s.done, s.result
	s.mu.Unlock()
	if done == nil {
		return errors.New("codex server has not been started")
	}
	<-done
	s.mu.Lock()
	defer s.mu.Unlock()
	return result.err
}

// Close requests shutdown and waits indefinitely for the process to exit.
// It is safe to call concurrently or before Start.
func (s *Server) Close() {
	s.mu.Lock()
	cmd, done := s.cmd, s.done
	if cmd == nil {
		s.mu.Unlock()
		return
	}
	if s.stopping {
		s.mu.Unlock()
		<-done
		return
	}
	s.stopping = true
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	s.mu.Unlock()
	<-done
	s.mu.Lock()
	s.cmd, s.stopping = nil, false
	s.mu.Unlock()
}

func NewCodexServer(
	rootDir string,
	user string,
	cfg *Config,
) (*Server, error) {
	workdir := filepath.Join(rootDir, user)
	if _, err := os.Stat(workdir); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		_ = os.MkdirAll(filepath.Join(workdir, ".codex"), os.ModePerm)

		if cfg != nil {
			content, err := toml.Marshal(cfg)
			if err != nil {
				return nil, err
			}
			_ = os.WriteFile(filepath.Join(workdir, ".codex", "config.toml"), content, 0644)
		}
	}

	return &Server{
		user:    user,
		workdir: workdir,
	}, nil
}
