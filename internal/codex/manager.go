package codex

import (
	"errors"
	"net/http"
	"sync"
)

const runtimeShardCount = 32

type Manager struct {
	shards    [runtimeShardCount]*serverShard
	closeOnce sync.Once
}

func (m *Manager) shard(user int) *serverShard {
	return m.shards[uint(user)%uint(len(m.shards))]
}

// Get returns the user's running server, or nil during startup and shutdown.
func (m *Manager) Get(user int) *UserRuntime {
	return m.shard(user).Get(user)
}

func (m *Manager) CreateOrGet(user int, headers http.Header) (*UserRuntime, error) {
	return m.shard(user).CreateOrGet(user, headers)
}

func (m *Manager) Destroy(user int) {
	m.shard(user).Destroy(user)
}

// Close drains in-flight operations and prevents subsequent creation.
func (m *Manager) Close() {
	m.closeOnce.Do(func() {
		for _, shard := range m.shards {
			shard.Close()
		}
	})
}

// NewRuntimeManager resolves each user to their remote Codex app-server URL.
func NewRuntimeManager(resolveEndpoint func(int) (string, error)) (*Manager, error) {
	if resolveEndpoint == nil {
		return nil, errors.New("Codex endpoint resolver is required")
	}

	m := new(Manager)
	for i := range m.shards {
		m.shards[i] = newServerShard(resolveEndpoint)
	}
	return m, nil
}

type serverShard struct {
	resolveEndpoint func(int) (string, error)
	mu              sync.RWMutex
	closed          bool
	servers         map[int]*UserRuntime
	// A transition lasts until startup or shutdown finishes. During shutdown
	// the old server stays registered so another destroy waits for that instance.
	transitions map[int]chan struct{}
}

func (s *serverShard) Get(user int) *UserRuntime {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.transitions[user] != nil {
		return nil
	}
	return s.servers[user]
}

func (s *serverShard) CreateOrGet(user int, headers http.Header) (*UserRuntime, error) {
	if server := s.Get(user); server != nil {
		return server, nil
	}
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, errors.New("runtime manager closed")
		}
		if wait := s.transitions[user]; wait != nil {
			s.mu.Unlock()
			<-wait
			continue
		}
		if server := s.servers[user]; server != nil {
			s.mu.Unlock()
			return server, nil
		}
		s.transitions[user] = make(chan struct{})
		s.mu.Unlock()

		endpoint, err := s.resolveEndpoint(user)
		var server *UserRuntime
		if err == nil {
			server, err = NewUserRuntime(endpoint, headers)
		}
		s.mu.Lock()
		if err == nil {
			s.servers[user] = server
		}
		s.finishTransitionLocked(user)
		s.mu.Unlock()
		if err != nil {
			return nil, err
		}
		go s.watch(user, server)
		return server, nil
	}
}

func (s *serverShard) finishTransitionLocked(user int) {
	close(s.transitions[user])
	delete(s.transitions, user)
}

func (s *serverShard) watch(user int, server *UserRuntime) {
	server.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.servers[user] == server && s.transitions[user] == nil {
		delete(s.servers, user)
	}
}

func (s *serverShard) Destroy(user int) {
	for {
		s.mu.Lock()
		server := s.servers[user]
		if wait := s.transitions[user]; wait != nil {
			s.mu.Unlock()
			<-wait
			if server != nil {
				return
			}
			continue
		}
		if server == nil {
			s.mu.Unlock()
			return
		}
		s.transitions[user] = make(chan struct{})
		s.mu.Unlock()

		server.Close()
		s.mu.Lock()
		delete(s.servers, user)
		s.finishTransitionLocked(user)
		s.mu.Unlock()
		return
	}
}

func (s *serverShard) Close() {
	s.mu.Lock()
	s.closed = true
	users := make([]int, 0, len(s.servers)+len(s.transitions))
	for user := range s.servers {
		users = append(users, user)
	}
	for user := range s.transitions {
		if s.servers[user] == nil {
			users = append(users, user)
		}
	}
	s.mu.Unlock()

	for _, user := range users {
		s.Destroy(user)
	}
}

func newServerShard(resolveEndpoint func(int) (string, error)) *serverShard {
	return &serverShard{
		resolveEndpoint: resolveEndpoint,
		servers:         make(map[int]*UserRuntime),
		transitions:     make(map[int]chan struct{}),
	}
}
