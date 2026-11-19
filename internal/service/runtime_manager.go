package service

import (
	"errors"
	"os"
	"strconv"
	"sync"

	"github.com/hjhsamuel/magent/internal/codex"
)

const runtimeShardCount = 32

type RuntimeManager struct {
	shards    [runtimeShardCount]*serverShard
	closeOnce sync.Once
}

func (m *RuntimeManager) shard(user int) *serverShard {
	return m.shards[uint(user)%uint(len(m.shards))]
}

// Get returns the user's running server, or nil during startup and shutdown.
func (m *RuntimeManager) Get(user int) *codex.Server {
	return m.shard(user).Get(user)
}

func (m *RuntimeManager) CreateOrGet(user int) (*codex.Server, error) {
	return m.shard(user).CreateOrGet(user)
}

func (m *RuntimeManager) Destroy(user int) {
	m.shard(user).Destroy(user)
}

// Close drains in-flight operations and prevents subsequent creation.
func (m *RuntimeManager) Close() {
	m.closeOnce.Do(func() {
		for _, shard := range m.shards {
			shard.Close()
		}
	})
}

func NewRuntimeManager(path string) (*RuntimeManager, error) {
	_ = os.MkdirAll(path, os.ModePerm)

	m := new(RuntimeManager)
	for i := range m.shards {
		m.shards[i] = newServerShard(path)
	}
	return m, nil
}

type serverShard struct {
	root    string
	mu      sync.RWMutex
	closed  bool
	servers map[int]*codex.Server
	// A transition lasts until startup or shutdown finishes. During shutdown
	// the old server stays registered so another destroy waits for that instance.
	transitions map[int]chan struct{}
}

func (s *serverShard) Get(user int) *codex.Server {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.transitions[user] != nil {
		return nil
	}
	return s.servers[user]
}

func (s *serverShard) CreateOrGet(user int) (*codex.Server, error) {
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

		server, err := codex.NewCodexServer(s.root, strconv.Itoa(user), nil)
		if err == nil {
			err = server.Start()
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

func (s *serverShard) watch(user int, server *codex.Server) {
	_ = server.Wait()
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

func newServerShard(root string) *serverShard {
	return &serverShard{
		root:        root,
		servers:     make(map[int]*codex.Server),
		transitions: make(map[int]chan struct{}),
	}
}
