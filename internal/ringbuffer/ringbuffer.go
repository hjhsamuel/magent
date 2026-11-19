// Package sse provides bounded, reconnectable single-producer/single-consumer streams.
package ringbuffer

import (
	"sync"
	"time"
)

// RingBuffer must not be copied. Its zero value is ready for use.
// IDs are assigned monotonically from 1. Data is owned by the caller and must
// remain immutable after publication, including any objects referenced by Data.
type RingBuffer struct {
	mu           sync.Mutex
	items        [Capacity]Payload
	last, marker uint64
	closed       bool
	producer     *Producer
	consumer     *Consumer
	idleTimer    *time.Timer
	unregister   func()
}

// Called with mu held whenever a managed stream becomes idle.
func (b *RingBuffer) startIdleTimer() {
	if b.unregister == nil || b.producer != nil || b.consumer != nil {
		return
	}
	var timer *time.Timer
	timer = time.AfterFunc(MaxIdleDuration, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		// A stopped callback may already be waiting for the lock.
		if b.idleTimer == timer {
			b.closeLocked()
		}
	})
	b.idleTimer = timer
}

func (b *RingBuffer) stopIdleTimer() {
	if b.idleTimer != nil {
		b.idleTimer.Stop()
		b.idleTimer = nil
	}
}

func (b *RingBuffer) ConnectProducer() (*Producer, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrClosed
	}
	if b.producer != nil {
		return nil, ErrProducerConnected
	}
	p := &Producer{b: b}
	b.producer = p
	b.stopIdleTimer()
	return p, nil
}

// ConnectConsumer resumes after lastID. Zero requests all retained messages.
// An overwritten ID resumes AT the latest retained part_end/complete marker,
// or at the oldest retained message if no marker survives. The same recovery
// applies when a connected consumer falls behind or lastID is ahead of the stream.
func (b *RingBuffer) ConnectConsumer(lastID uint64) (*Consumer, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrClosed
	}
	if b.consumer != nil {
		return nil, ErrConsumerConnected
	}

	c := &Consumer{
		b:    b,
		next: lastID + 1,
		wake: make(chan struct{}, 1),
	}
	if lastID == 0 {
		c.next = b.oldest()
	} else if lastID > b.last || lastID < b.oldest() {
		c.next = b.recovery()
	}
	b.consumer = c
	b.stopIdleTimer()
	return c, nil
}

func (b *RingBuffer) oldest() uint64 {
	if b.last >= Capacity {
		return b.last - Capacity + 1
	}
	return 1
}

func (b *RingBuffer) recovery() uint64 {
	if b.marker >= b.oldest() {
		return b.marker
	}
	return b.oldest()
}

// Close immediately terminates the stream and releases retained payloads.
func (b *RingBuffer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closeLocked()
}

func (b *RingBuffer) closeLocked() {
	b.stopIdleTimer()
	if b.unregister != nil {
		b.unregister()
		b.unregister = nil
	}
	b.closed = true
	if b.consumer != nil {
		b.consumer.signal()
	}
	b.consumer, b.producer = nil, nil
	clear(b.items[:])
}

func NewRingBuffer() *RingBuffer {
	return &RingBuffer{
		items: [Capacity]Payload{},
	}
}
