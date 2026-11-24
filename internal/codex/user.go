package codex

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

// UserRuntime owns one user's active runtimes, keyed by thread ID.
type UserRuntime struct {
	endpoint string
	headers  http.Header
	mu       sync.Mutex
	closed   bool
	done     chan struct{}
	runtimes map[string]*Runtime
}

func (s *UserRuntime) Wait() {
	<-s.done
}

func (s *UserRuntime) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	for id, r := range s.runtimes {
		_ = r.Close()
		delete(s.runtimes, id)
	}
	close(s.done)
}

func (s *UserRuntime) connect(ctx context.Context) (*Runtime, error) {
	if s.closed {
		return nil, errors.New("codex server closed")
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	return NewRuntimeContext(ctx, s.endpoint, s.headers)
}

// GetRuntime returns the active connection for threadID, or nil if unavailable.
func (s *UserRuntime) GetRuntime(threadID string) *Runtime {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	return s.activeRuntimeLocked(threadID)
}

// activeRuntimeLocked is called with mu held.
func (s *UserRuntime) activeRuntimeLocked(threadID string) *Runtime {
	r := s.runtimes[threadID]
	if r != nil {
		select {
		case <-r.Done():
			delete(s.runtimes, threadID)
			return nil
		default:
		}
	}
	return r
}

// OpenRuntime creates a thread when threadID is empty, otherwise resumes it.
// Repeated opens of an active thread return its existing Runtime.
func (s *UserRuntime) OpenRuntime(ctx context.Context, threadID string) (*Runtime, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errors.New("codex server closed")
	}

	if r := s.activeRuntimeLocked(threadID); r != nil {
		return r, nil
	}

	r, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	threadID, err = r.OpenThread(ctx, threadID)
	if err != nil {
		r.Close()
		return nil, err
	}
	s.runtimes[threadID] = r

	go func() {
		r.WaitClosed()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.runtimes[threadID] == r {
			delete(s.runtimes, threadID)
		}
	}()

	return r, nil
}

// RemoveRuntime closes and removes an active connection without deleting the
// persisted thread. Missing entries are a no-op. Keep the lock until Close
// finishes so concurrent opens cannot replace a connection still shutting down.
func (s *UserRuntime) RemoveRuntime(threadID string) error {
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

// ReloadSkillsAndMCPServers requests both reloads over one temporary connection.
func (s *UserRuntime) ReloadSkillsAndMCPServers(ctx context.Context, mcp, skill bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer r.Close()

	go func() {
		for range r.Events() {
		}
	}()

	if skill {
		if err = r.ReloadSkills(ctx); err != nil {
			return err
		}
	}
	if mcp {
		if err = r.ReloadMCPServers(ctx); err != nil {
			return err
		}
	}
	return nil
}

// ThreadStatuses uses one temporary connection and never resumes the threads.
// On failure the returned map contains only successfully queried statuses.
func (s *UserRuntime) ThreadStatuses(ctx context.Context, threadIDs ...string) (map[string]ThreadStatus, error) {
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

		response, err := r.ThreadRead(ctx, ThreadReadParams{ThreadID: id})
		if err != nil {
			return statuses, fmt.Errorf("read thread %q: %w", id, err)
		}
		statuses[id] = response.Thread.Status
	}
	return statuses, nil
}

// ThreadTurns requests the latest limit turns once per distinct thread over one temporary connection.
// On failure it returns the results for previously completed threads.
func (s *UserRuntime) ThreadTurns(ctx context.Context, limit int, threadIDs ...string) (map[string][]Turn, error) {
	results := make(map[string][]Turn, len(threadIDs))
	if len(threadIDs) == 0 {
		return results, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
		if _, ok := results[id]; ok {
			continue
		}
		params := ThreadTurnsListParams{
			ThreadID:      id,
			SortDirection: "desc",
			ItemsView:     "summary",
			Limit:         limit,
		}
		var response ThreadTurnsListResponse
		if err := r.call(ctx, MethodThreadTurnsList, params, &response); err != nil {
			return results, fmt.Errorf("list turns for thread %q: %w", id, err)
		}
		if response.Data == nil {
			return results, fmt.Errorf("list turns for thread %q: missing data", id)
		}
		results[id] = response.Data
	}
	return results, nil
}

func NewUserRuntime(endpoint string, headers http.Header) (*UserRuntime, error) {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Hostname() == "" || u.Fragment != "" || u.User != nil {
		return nil, errors.New("expected a ws:// or wss:// Codex endpoint")
	}
	return &UserRuntime{
		endpoint: endpoint,
		headers:  headers.Clone(),
		done:     make(chan struct{}),
		runtimes: make(map[string]*Runtime),
	}, nil
}
