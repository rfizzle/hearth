package orchestrate

import (
	"context"
	"sync"
	"time"

	"github.com/rfizzle/hearth/internal/org"
)

const (
	// ringSize is how many of the latest events the bus keeps for a
	// subscriber that reconnects.
	ringSize = 4096
	// subscriberBuffer is how far a subscriber may fall behind before it is
	// dropped: a whole ring's replay with room for live events behind it.
	subscriberBuffer = ringSize + 512
	// heartbeatEvery keeps a quiet subscription under the client's idle
	// timeout.
	heartbeatEvery = 20 * time.Second
)

// Event is one change, numbered in the order it was published. Body is one
// of the event types below, or a type another publisher defines.
type Event struct {
	Seq  uint64
	Body any
}

// MessageDelta is text appended to a streaming message. Deltas are never
// stored; the message's final MessageUpserted carries the whole text.
type MessageDelta struct {
	ConversationID string
	MessageID      string
	RunID          string
	Text           string
}

// MessageUpserted is a message created or replaced whole.
type MessageUpserted struct {
	Message org.Message
}

// MessageDeleted is a message removed from its conversation.
type MessageDeleted struct {
	ConversationID string
	MessageID      string
}

// RunChanged is a run's new state or step count.
type RunChanged struct {
	Run org.Run
}

// Resync tells a subscriber that events it asked for are no longer kept: it
// refetches what it shows and continues from this event's sequence.
type Resync struct{}

// Heartbeat is sent to a quiet subscription. Its sequence is the latest
// published, every event up to which the subscriber has already been sent.
type Heartbeat struct{}

// Bus numbers events and hands them to every subscriber, keeping the latest
// for one that reconnects. The zero value is not ready; use NewBus.
type Bus struct {
	mu   sync.Mutex
	seq  uint64
	ring [ringSize]Event // the event with sequence s is at (s-1) % ringSize
	subs map[chan Event]struct{}
}

// NewBus is an empty bus whose first event is sequence 1.
func NewBus() *Bus {
	return &Bus{subs: map[chan Event]struct{}{}}
}

// Publish numbers body, keeps it and sends it to every subscriber, dropping
// any subscriber whose buffer is full rather than waiting for it. It returns
// the event as sent.
func (b *Bus) Publish(body any) Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	e := Event{Seq: b.seq, Body: body}
	b.ring[(e.Seq-1)%ringSize] = e
	b.send(e)
	return e
}

// send hands e to every subscriber. The lock is held.
func (b *Bus) send(e Event) {
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			delete(b.subs, ch)
			close(ch)
		}
	}
}

// Subscribe follows the bus from after: every kept event after it, then
// every event published from now on. An after of zero asks for live events
// only. When events after it are no longer kept, or after is from an earlier
// launch, the first event is a Resync and live events follow it. The
// channel is closed when cancel is called or the subscriber is dropped for
// falling behind.
func (b *Bus) Subscribe(after uint64) (events <-chan Event, cancel func()) {
	ch := make(chan Event, subscriberBuffer)
	b.mu.Lock()
	defer b.mu.Unlock()
	oldest := uint64(1)
	if b.seq > ringSize {
		oldest = b.seq - ringSize + 1
	}
	switch {
	case after == 0:
	case after > b.seq || after+1 < oldest:
		ch <- Event{Seq: b.seq, Body: Resync{}}
	default:
		for s := after + 1; s <= b.seq; s++ {
			ch <- b.ring[(s-1)%ringSize]
		}
	}
	b.subs[ch] = struct{}{}
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
	}
}

// Heartbeats sends a Heartbeat to every subscriber every 20 seconds until
// ctx ends. A heartbeat is not kept and takes no sequence of its own.
func (b *Bus) Heartbeats(ctx context.Context) {
	t := time.NewTicker(heartbeatEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.mu.Lock()
			b.send(Event{Seq: b.seq, Body: Heartbeat{}})
			b.mu.Unlock()
		}
	}
}
