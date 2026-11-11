package runtime

import (
	"errors"
	"hash/fnv"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	ErrRuntimeManagerClosed = errors.New("runtime manager is closed")
	ErrRuntimeExists        = errors.New("runtime ID already exists")
	ErrInvalidRuntime       = errors.New("runtime ID and runtime are required")
)

// Manager owns registered connections. Its zero value is ready for use.
// Register transfers ownership only on success; register each runtime once.
// Network operations never run while a shard lock is held.
type Manager struct {
	shards    [16]runtimeShard
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

func NewRuntimeManager() *Manager {
	return &Manager{}
}

func (m *Manager) shard(id string) *runtimeShard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return &m.shards[h.Sum32()%uint32(len(m.shards))]
}

// Register wraps an existing runtime (initialization may be performed through
// the returned wrapper). Only consume the wrapper's Events after registration.
func (m *Manager) Register(id string, r Runtime) (*ManagedRuntime, error) {
	if id == "" || r == nil {
		return nil, ErrInvalidRuntime
	}

	shard := m.shard(id)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if m.closed.Load() {
		return nil, ErrRuntimeManagerClosed
	}
	if _, ok := shard.connections[id]; ok {
		return nil, ErrRuntimeExists
	}
	if shard.connections == nil {
		shard.connections = make(map[string]*ManagedRuntime)
	}

	c := &ManagedRuntime{
		Runtime: r,
		events:  make(chan Event, 256),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	shard.connections[id] = c

	go func() {
		defer close(c.done)
		defer func() {
			shard.mu.Lock()
			if shard.connections[id] == c {
				delete(shard.connections, id)
			}
			shard.mu.Unlock()
		}()

		c.forward()
	}()

	return c, nil
}

func (m *Manager) Get(id string) (*ManagedRuntime, bool) {
	shard := m.shard(id)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	r, ok := shard.connections[id]
	return r, ok
}

// Remove closes the connection and waits for its event worker to exit.
func (m *Manager) Remove(id string) error {
	shard := m.shard(id)
	shard.mu.Lock()
	r := shard.connections[id]
	delete(shard.connections, id)
	shard.mu.Unlock()

	if r == nil {
		return nil
	}
	return r.Close()
}

// Close rejects new registrations and closes all connections concurrently.
func (m *Manager) Close() error {
	m.closeOnce.Do(func() {
		// Register checks closed under its shard lock: racing registrations
		// are either rejected or included when that shard is drained.
		m.closed.Store(true)

		var connections []*ManagedRuntime
		for i := range m.shards {
			shard := &m.shards[i]
			shard.mu.Lock()
			for _, c := range shard.connections {
				connections = append(connections, c)
			}
			shard.connections = nil
			shard.mu.Unlock()
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, c := range connections {
			wg.Add(1)
			go func() {
				defer wg.Done()

				if err := c.Close(); err != nil {
					mu.Lock()
					m.closeErr = errors.Join(m.closeErr, err)
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
	})
	return m.closeErr
}

type runtimeShard struct {
	mu          sync.RWMutex
	connections map[string]*ManagedRuntime
}

// ManagedRuntime preserves the Runtime API. Events has a bounded buffer:
// callers must consume it continuously; slow consumers only stall their own
// connection. On turn.completed, Text is the last assistant message, not a
// generated summary. Status and Error retain failed/interrupted outcomes.
// Closing or disconnecting never synthesizes a successful completion.
type ManagedRuntime struct {
	Runtime
	events   chan Event
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
	closeErr error
}

func (r *ManagedRuntime) Events() <-chan Event {
	return r.events
}

func (r *ManagedRuntime) Done() <-chan struct{} {
	return r.done
}

func (r *ManagedRuntime) shutdown() {
	r.once.Do(func() {
		close(r.stop)
		r.closeErr = r.Runtime.Close()
	})
}

func (r *ManagedRuntime) Close() error {
	r.shutdown()
	<-r.done
	return r.closeErr
}

type turnKey struct {
	session string
	turn    string
}

type messageText struct {
	text     strings.Builder
	complete bool
}

type turnText struct {
	items map[string]*messageText
	last  string
}

func (r *ManagedRuntime) forward() {
	defer close(r.events)
	defer r.shutdown()

	turns := make(map[turnKey]*turnText)
	source := r.Runtime.Events()
	for {
		select {
		case <-r.stop:
			return
		case <-r.Runtime.Done():
			return
		case e, ok := <-source:
			if !ok {
				return
			}

			key := turnKey{
				e.SessionID,
				e.TurnID,
			}
			switch e.Type {
			case EventTextDelta, EventTextCompleted:
				t := turns[key]
				if t == nil {
					t = &turnText{items: make(map[string]*messageText)}
					turns[key] = t
				}
				item := t.items[e.ItemID]
				if item == nil {
					item = &messageText{}
					t.items[e.ItemID] = item
					t.last = e.ItemID
				}
				if e.Type == EventTextCompleted {
					item.text.Reset()
					item.text.WriteString(e.Text)
					item.complete = true
				} else if !item.complete {
					item.text.WriteString(e.Text)
				}
			case EventTurnCompleted:
				if t := turns[key]; t != nil {
					e.Text = t.items[t.last].text.String()
				}
				delete(turns, key)
			case EventSessionClosed:
				for k := range turns {
					if k.session == e.SessionID {
						delete(turns, k)
					}
				}
			}

			select {
			case r.events <- e:
			case <-r.stop:
				return
			case <-r.Runtime.Done():
				return
			}
		}
	}
}
