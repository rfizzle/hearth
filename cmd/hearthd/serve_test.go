package main

import (
	"context"
	"io"
	"testing"
	"time"
)

func TestLifeline(t *testing.T) {
	tests := []struct {
		name      string
		close     func(w *io.PipeWriter)
		cancelled bool
	}{
		{name: "open pipe keeps it alive", close: func(*io.PipeWriter) {}, cancelled: false},
		{name: "EOF cancels", close: func(w *io.PipeWriter) { _ = w.Close() }, cancelled: true},
		{name: "a read error cancels", close: func(w *io.PipeWriter) { _ = w.CloseWithError(io.ErrUnexpectedEOF) }, cancelled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w := io.Pipe()
			t.Cleanup(func() { _ = w.Close() })
			ctx := lifeline(context.Background(), r)
			if _, err := w.Write([]byte("ignored")); err != nil {
				t.Fatalf("write: %v", err)
			}
			if ctx.Err() != nil {
				t.Fatal("cancelled before the pipe closed")
			}
			tt.close(w)
			if tt.cancelled {
				<-ctx.Done()
				return
			}
			select {
			case <-ctx.Done():
				t.Fatal("cancelled while the pipe is open")
			case <-time.After(10 * time.Millisecond):
			}
		})
	}
}
