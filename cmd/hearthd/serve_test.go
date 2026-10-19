package main

import (
	"bufio"
	"context"
	"io"
	"strings"
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

func TestTokenLineThenLifeline(t *testing.T) {
	r, w := io.Pipe()
	t.Cleanup(func() { _ = w.Close() })
	br := bufio.NewReader(r)
	go func() { _, _ = w.Write([]byte("s3cret\nmore\n")) }()

	tok, err := readToken(br)
	if err != nil || tok != "s3cret" {
		t.Fatalf("readToken = %q, %v; want s3cret", tok, err)
	}
	ctx := lifeline(context.Background(), br)
	select {
	case <-ctx.Done():
		t.Fatal("cancelled while the pipe is open")
	case <-time.After(10 * time.Millisecond):
	}
	_ = w.Close()
	<-ctx.Done()
}

func TestReadToken(t *testing.T) {
	tests := []struct {
		name, in, want string
		wantErr        bool
	}{
		{name: "line", in: "abc\nrest", want: "abc"},
		{name: "crlf", in: "abc\r\n", want: "abc"},
		{name: "no newline at EOF", in: "abc", want: "abc"},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readToken(bufio.NewReader(strings.NewReader(tt.in)))
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("readToken = %q, %v; want %q, err %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
