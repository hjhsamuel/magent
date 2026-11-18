package ringbuffer

import "sync"

// Manager owns streams and expires them after ten continuous seconds idle.
// Its zero value is usable.
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

	return m.createLocked(id), nil
}

func (m *Manager) createLocked(id string) *RingBuffer {
	if m.streams == nil {
		m.streams = make(map[string]*RingBuffer)
	}

	b := NewRingBuffer()
	b.mu.Lock()
	b.unregister = func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.streams[id] == b {
			delete(m.streams, id)
		}
	}
	b.startIdleTimer()
	b.mu.Unlock()
	m.streams[id] = b
	return b
}

func (m *Manager) Get(id string) (*RingBuffer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	b, ok := m.streams[id]
	return b, ok
}

// CreateOrGet atomically returns the existing stream or creates one.
// A closed Manager returns ErrClosed and cannot create new streams.
func (m *Manager) CreateOrGet(id string) (*RingBuffer, error) {
	// Existing streams only need a shared lock. Recheck after taking the write
	// lock because another caller may create the stream or close the manager.
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, ErrClosed
	}
	b, ok := m.streams[id]
	m.mu.RUnlock()
	if ok {
		return b, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, ErrClosed
	}
	if b, ok := m.streams[id]; ok {
		return b, nil
	}
	return m.createLocked(id), nil
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
