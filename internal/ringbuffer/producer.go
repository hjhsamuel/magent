package ringbuffer

type Producer struct {
	b *RingBuffer
}

// Write never waits for the consumer. The supplied ID is replaced by the assigned ID.
// complete is a replay marker; it does not close the stream.
func (p *Producer) Write(event Payload) (uint64, error) {
	b := p.b

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed || b.producer != p {
		return 0, ErrClosed
	}

	b.last++
	event.ID = b.last

	b.items[(b.last-1)&(Capacity-1)] = event
	if event.Type == SSEventPartEnd || event.Type == SSEventComplete {
		b.marker = b.last
	}
	if b.consumer != nil {
		b.consumer.signal()
	}
	return b.last, nil
}

// Close releases the producer slot without destroying replay history.
func (p *Producer) Close() {
	p.b.mu.Lock()
	defer p.b.mu.Unlock()

	if p.b.producer == p {
		p.b.producer = nil
	}
}
