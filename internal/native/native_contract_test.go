//go:build contract

package native

import (
	"context"
	"os"
	"testing"
)

// TestAgainstTheApp is the hand check against the running app. It runs only
// when HEARTH_NATIVE_ADDR (the app's native address) and HEARTH_NATIVE_TOKEN
// (the launch token) are set; HEARTH_NATIVE_FILE, an absolute path to a scan
// or a screenshot, adds a read of its first page.
func TestAgainstTheApp(t *testing.T) {
	addr, token := os.Getenv("HEARTH_NATIVE_ADDR"), os.Getenv("HEARTH_NATIVE_TOKEN")
	if addr == "" {
		t.Skip("HEARTH_NATIVE_ADDR is not set; this is a hand check against the running app")
	}
	c, err := New(addr, token)
	if err != nil {
		t.Fatal(err)
	}
	version, err := c.Ping(context.Background())
	if err != nil || version < 1 {
		t.Fatalf("ping: %d, %v", version, err)
	}
	path := os.Getenv("HEARTH_NATIVE_FILE")
	if path == "" {
		return
	}
	pages, err := c.Recognise(context.Background(), path, []int{1})
	if err != nil {
		t.Fatalf("recognise %s: %v", path, err)
	}
	for _, p := range pages {
		t.Logf("page %d (text layer %v, labels %v): %d bytes of text", p.Page, p.FromTextLayer, p.Labels, len(p.Text))
	}
}
