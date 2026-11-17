package ringbuffer

import "sync"

// Manager owns streams until Delete or Close. Its zero value is usable.
// It has no per-stream timers or goroutines; owners delete expired streams.
// Stream reads and writes never acquire the manager lock.
type Manager struct {
	mu      sync.RWMutex
	streams map[string]*RingBuffer
	closed  bool
}

func (m *Manager) Create(id string) (*RingBuffer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, ErrClosed
	}
	if _, ok := m.streams[id]; ok {
		return nil, ErrExists
	}

	if m.streams == nil {
		m.streams = make(map[string]*RingBuffer)
	}

	b := NewRingBuffer()
	m.streams[id] = b
	return b, nil
}

func (m *Manager) Get(id string) (*RingBuffer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	b, ok := m.streams[id]
	return b, ok
}

func (m *Manager) Delete(id string) {
	m.mu.Lock()
	b := m.streams[id]
	delete(m.streams, id)
	m.mu.Unlock()

	if b != nil {
		b.Close()
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	streams := m.streams
	m.streams = nil
	m.mu.Unlock()

	for _, b := range streams {
		b.Close()
	}
}

func NewManager() *Manager {
	return &Manager{
		streams: make(map[string]*RingBuffer),
	}
}
