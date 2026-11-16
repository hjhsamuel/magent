//go:build linux

package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Server owns one user's process and active thread connections.
type Server struct {
	user, workdir, socket string
	config                []byte
	mu                    sync.Mutex
	cmd                   *exec.Cmd
	done                  chan struct{}
	stopped               chan struct{}
	result                *serverResult
	stopping              bool
	runtimes              map[string]*Runtime
}

// Start initializes the workspace then launches the process. Connections may be
// opened immediately: connect retries until the socket is ready or ctx expires.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil || s.stopping {
		return errors.New("codex server already running or stopping")
	}

	home := filepath.Join(s.workdir, ".codex")
	_ = os.MkdirAll(home, os.ModePerm)
	if s.config != nil {
		_ = os.WriteFile(filepath.Join(home, "config.toml"), s.config, 0644)
	}

	cmd := exec.Command("codex", "app-server", "--listen", "unix://"+s.socket)
	cmd.Dir = s.workdir
	cmd.Env = append(os.Environ(), "CODEX_HOME="+home)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start codex server: %w", err)
	}

	s.cmd, s.done, s.stopped, s.result = cmd, make(chan struct{}), make(chan struct{}), &serverResult{}
	done, result := s.done, s.result
	go func() {
		err := cmd.Wait()

		s.mu.Lock()
		result.err = err
		close(done)
		unexpected := !s.stopping
		s.mu.Unlock()

		if unexpected {
			s.closeProcess(cmd)
		}
	}()

	return nil
}

func (s *Server) Wait() error {
	s.mu.Lock()
	stopped, result := s.stopped, s.result
	s.mu.Unlock()
	if stopped == nil {
		return errors.New("codex server has not been started")
	}
	<-stopped
	return result.err
}

// Close sends SIGTERM first, then waits for Runtime connections to close
// and for the process to be reaped. Runtime shutdown is driven by the server.
// It waits indefinitely for process exit; shutdown never escalates to SIGKILL.
// Concurrent callers wait for the same complete shutdown.
func (s *Server) Close() {
	s.closeProcess(nil)
}

func (s *Server) closeProcess(expected *exec.Cmd) {
	s.mu.Lock()
	if s.cmd == nil || (expected != nil && s.cmd != expected) {
		s.mu.Unlock()
		return
	}
	if s.stopping {
		stopped := s.stopped
		s.mu.Unlock()
		<-stopped
		return
	}
	s.stopping = true
	cmd, done := s.cmd, s.done
	children := make([]*Runtime, 0, len(s.runtimes))
	for _, r := range s.runtimes {
		children = append(children, r)
	}
	s.mu.Unlock()
	// Signal before waiting: app-server closes the connections as it shuts down,
	// allowing each Runtime read loop to finish and deliver its remaining events.
	select {
	case <-done:
	default:
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	for _, r := range children {
		r.waitClosed()
	}
	<-done
	s.mu.Lock()
	s.runtimes = make(map[string]*Runtime)
	s.cmd = nil
	s.stopping = false
	close(s.stopped)
	s.mu.Unlock()
}

// connect is called with mu held, serializing creation with shutdown.
func (s *Server) connect(ctx context.Context) (*Runtime, error) {
	if s.cmd == nil || s.stopping {
		return nil, errors.New("codex server is not running")
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	for {
		select {
		case <-s.done:
			return nil, errors.New("codex server exited")
		default:
		}
		c, err := NewUnixConnContext(ctx, s.socket)
		if err == nil {
			r := newRuntime(c)
			if err = r.init(ctx); err != nil {
				r.Close()
				return nil, err
			}
			return r, nil
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("connect codex server: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

// OpenRuntime creates a thread when threadID is empty, otherwise resumes it.
// Repeated opens of an active thread return its existing Runtime.
func (s *Server) OpenRuntime(ctx context.Context, threadID string) (*Runtime, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping {
		return nil, errors.New("codex server stopping")
	}
	if r := s.runtimes[threadID]; r != nil {
		select {
		case <-r.Done():
			delete(s.runtimes, threadID)
		default:
			return r, nil
		}
	}
	r, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	var response ThreadResponse
	if threadID == "" {
		response, err = r.ThreadStart(ctx, ThreadStartParams{})
	} else {
		response, err = r.ThreadResume(ctx, ThreadResumeParams{ThreadID: threadID})
	}
	if err != nil {
		r.Close()
		return nil, err
	}
	r.threadID = response.Thread.ID
	s.runtimes[r.threadID] = r
	go func() {
		r.waitClosed()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.runtimes[r.threadID] == r {
			delete(s.runtimes, r.threadID)
		}
	}()
	return r, nil
}

// RemoveRuntime closes and removes an active connection without deleting the
// persisted thread. Missing entries are a no-op. Keep the lock until Close
// finishes so concurrent opens cannot replace a connection still shutting down.
func (s *Server) RemoveRuntime(threadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.runtimes[threadID]
	if r == nil {
		return nil
	}
	if err := r.Close(); err != nil {
		return err
	}
	delete(s.runtimes, threadID)
	return nil
}

// ThreadStatuses uses one temporary connection and never resumes the threads.
// On failure the returned map contains only successfully queried statuses.
func (s *Server) ThreadStatuses(ctx context.Context, threadIDs []string) (map[string]ThreadStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	statuses := make(map[string]ThreadStatus, len(threadIDs))
	if len(threadIDs) == 0 {
		return statuses, nil
	}
	r, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	go func() {
		for range r.Events() {
		}
	}()
	for _, id := range threadIDs {
		if _, ok := statuses[id]; ok {
			continue
		}

		var response ThreadResponse
		if err = r.call(
			ctx,
			MethodThreadRead,
			ThreadReadParams{
				ThreadID: id,
			},
			&response,
		); err != nil {
			return statuses, fmt.Errorf("read thread %q: %w", id, err)
		}

		if response.Thread.ID != id || response.Thread.Status.Type == "" {
			return statuses, fmt.Errorf("invalid thread/read response for %q", id)
		}
		statuses[id] = response.Thread.Status
	}
	return statuses, nil
}

func NewCodexServer(rootDir, user string, cfg *Config) (*Server, error) {
	if user == "" || user == "." || user == ".." || strings.ContainsAny(user, "/\\") {
		return nil, errors.New("user must be a single directory name")
	}
	workdir, err := filepath.Abs(filepath.Join(rootDir, user))
	if err != nil {
		return nil, err
	}
	var content []byte
	if cfg != nil {
		content, err = toml.Marshal(cfg)
		if err != nil {
			return nil, err
		}
	}
	return &Server{
		user:     user,
		workdir:  workdir,
		socket:   filepath.Join("/run/codex", user+".sock"),
		config:   content,
		runtimes: make(map[string]*Runtime),
	}, nil
}

type serverResult struct {
	err error
}
