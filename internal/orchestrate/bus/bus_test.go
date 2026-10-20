package bus

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

// drain reads what is waiting on ch without blocking.
func drain(ch <-chan Event) []Event {
	var got []Event
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, e)
		default:
			return got
		}
	}
}

func publishN(b *Bus, n int) {
	for i := range n {
		b.Publish(i)
	}
}

func TestBus(t *testing.T) {
	t.Run("a subscriber is replayed what it missed, then follows live", func(t *testing.T) {
		b := New()
		publishN(b, 5)
		ch, cancel := b.Subscribe(3)
		defer cancel()
		b.Publish("live")
		got := drain(ch)
		if len(got) != 3 || got[0].Seq != 4 || got[1].Seq != 5 || got[2].Seq != 6 || got[2].Body != "live" {
			t.Fatalf("events = %+v, want 4, 5 and the live 6", got)
		}
	})

	t.Run("after zero follows live events only", func(t *testing.T) {
		b := New()
		publishN(b, 5)
		ch, cancel := b.Subscribe(0)
		defer cancel()
		if got := drain(ch); len(got) != 0 {
			t.Fatalf("replayed %+v to a live-only subscriber", got)
		}
		b.Publish("live")
		if got := drain(ch); len(got) != 1 || got[0].Seq != 6 {
			t.Fatalf("events = %+v, want the live 6", got)
		}
	})

	t.Run("a subscriber 5,000 events behind gets Resync first", func(t *testing.T) {
		b := New()
		publishN(b, 5001)
		ch, cancel := b.Subscribe(1)
		defer cancel()
		b.Publish("live")
		got := drain(ch)
		if len(got) != 2 {
			t.Fatalf("events = %d, want a resync and the live event", len(got))
		}
		if _, ok := got[0].Body.(Resync); !ok || got[0].Seq != 5001 {
			t.Fatalf("first event = %+v, want Resync at 5001", got[0])
		}
		if got[1].Seq != 5002 {
			t.Fatalf("second event = %+v, want the live 5002", got[1])
		}
	})

	t.Run("the oldest kept event is replayed without a resync", func(t *testing.T) {
		b := New()
		publishN(b, ringSize+10)
		ch, cancel := b.Subscribe(10)
		defer cancel()
		got := drain(ch)
		if len(got) != ringSize || got[0].Seq != 11 {
			t.Fatalf("replayed %d events from %d, want %d from 11", len(got), got[0].Seq, ringSize)
		}
	})

	t.Run("a sequence from an earlier launch gets Resync", func(t *testing.T) {
		b := New()
		publishN(b, 3)
		ch, cancel := b.Subscribe(900)
		defer cancel()
		got := drain(ch)
		if len(got) != 1 || got[0].Seq != 3 {
			t.Fatalf("events = %+v, want one Resync at 3", got)
		}
		if _, ok := got[0].Body.(Resync); !ok {
			t.Fatalf("event = %+v, want Resync", got[0])
		}
	})

	t.Run("a slow subscriber is dropped and the publisher does not wait", func(t *testing.T) {
		b := New()
		slow, cancelSlow := b.Subscribe(0)
		defer cancelSlow()
		fast, cancelFast := b.Subscribe(0)
		defer cancelFast()
		for i := range subscriberBuffer + 10 {
			b.Publish(i)
			drain(fast)
		}
		n := 0
		for range slow {
			n++
		}
		if n != subscriberBuffer {
			t.Fatalf("slow subscriber read %d before its channel closed, want %d", n, subscriberBuffer)
		}
		b.Publish("after")
		if got := drain(fast); len(got) != 1 {
			t.Fatalf("fast subscriber = %+v, want it still following", got)
		}
	})

	t.Run("cancel closes the channel and is safe to repeat", func(t *testing.T) {
		b := New()
		ch, cancel := b.Subscribe(0)
		cancel()
		cancel()
		if _, ok := <-ch; ok {
			t.Fatal("channel still open after cancel")
		}
		b.Publish("nobody")
	})

	t.Run("a quiet subscription gets a heartbeat every 20 seconds", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b := New()
			publishN(b, 2)
			ch, cancel := b.Subscribe(0)
			defer cancel()
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			go b.Heartbeats(ctx)

			time.Sleep(19 * time.Second)
			synctest.Wait()
			if got := drain(ch); len(got) != 0 {
				t.Fatalf("events before 20s = %+v", got)
			}
			time.Sleep(time.Second)
			synctest.Wait()
			got := drain(ch)
			if len(got) != 1 || got[0].Seq != 2 {
				t.Fatalf("events = %+v, want one heartbeat at the latest sequence", got)
			}
			if _, ok := got[0].Body.(Heartbeat); !ok {
				t.Fatalf("event = %+v, want Heartbeat", got[0])
			}
			b.Publish("next")
			if got := drain(ch); len(got) != 1 || got[0].Seq != 3 {
				t.Fatalf("events after the heartbeat = %+v, want 3: a heartbeat takes no sequence", got)
			}
		})
	})
}
