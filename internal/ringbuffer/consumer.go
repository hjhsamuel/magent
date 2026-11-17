package ringbuffer

import (
	"context"
	"io"
)

type Consumer struct {
	b    *RingBuffer
	next uint64
	wake chan struct{}
}

func (c *Consumer) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Next waits for a message or cancellation. Calls on a consumer should be serial.
func (c *Consumer) Next(ctx context.Context) (Payload, error) {
	b := c.b
	for {
		if err := ctx.Err(); err != nil {
			return Payload{}, err
		}

		b.mu.Lock()
		if b.closed {
			b.mu.Unlock()
			return Payload{}, io.EOF
		}
		if b.consumer != c {
			b.mu.Unlock()
			return Payload{}, ErrClosed
		}
		if c.next < b.oldest() {
			c.next = b.recovery()
		}
		if c.next <= b.last {
			event := b.items[(c.next-1)&(Capacity-1)]
			c.next++
			b.mu.Unlock()
			return event, nil
		}
		b.mu.Unlock()

		select {
		case <-ctx.Done():
			return Payload{}, ctx.Err()
		case <-c.wake:
		}
	}
}

// ReadAll returns all currently unread events in order and advances the cursor.
// It never waits for new events; an empty stream returns nil, nil. Overwritten
// events use the same marker recovery as Next. Calls on a consumer should be
// serial. The returned slice is independent of the ring; Data remains shared
// and must be treated as immutable.
func (c *Consumer) ReadAll() ([]Payload, error) {
	b := c.b

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, io.EOF
	}
	if b.consumer != c {
		return nil, ErrClosed
	}
	if c.next < b.oldest() {
		c.next = b.recovery()
	}
	if c.next > b.last {
		return nil, nil
	}

	events := make([]Payload, int(b.last-c.next+1))
	start := int((c.next - 1) & (Capacity - 1))
	n := copy(events, b.items[start:])
	copy(events[n:], b.items[:])
	c.next = b.last + 1

	return events, nil
}

func (c *Consumer) Close() {
	c.b.mu.Lock()
	defer c.b.mu.Unlock()

	if c.b.consumer == c {
		c.b.consumer = nil
		c.signal()
	}
}
