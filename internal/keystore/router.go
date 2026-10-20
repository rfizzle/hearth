package keystore

import (
	"sync"

	"github.com/rfizzle/llm/chatgpt/credstore"
)

// guarded is a store behind a read-write lock: a call holds the read lock for
// its whole run, and a replacement holds the write lock while it works, so a
// call is wholly before or wholly after it.
type guarded[T any] struct {
	mu  sync.RWMutex
	cur T
}

// hold takes the read lock and returns the current store and the release.
func (g *guarded[T]) hold() (T, func()) {
	g.mu.RLock()
	return g.cur, g.mu.RUnlock
}

func (g *guarded[T]) replace(fn func(cur T) (T, error)) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	next, err := fn(g.cur)
	if err == nil {
		g.cur = next
	}
	return err
}

// Router is the Store every holder of a shared key store keeps. Replace points
// it at another store, so the holders (the integrations, say) need not be
// told when the core moves from its files to memory.
type Router struct{ g guarded[Store] }

// NewRouter is a Router over cur.
func NewRouter(cur Store) *Router { return &Router{g: guarded[Store]{cur: cur}} }

// Replace runs fn with every call held off, and makes the store it returns
// the current one unless it returns an error. A call already in flight on
// the old store finishes first.
func (r *Router) Replace(fn func(cur Store) (Store, error)) error { return r.g.replace(fn) }

// Load, Save, Clear and Names go to the current store.
func (r *Router) Load(name string) (string, error) {
	s, done := r.g.hold()
	defer done()
	return s.Load(name)
}

func (r *Router) Save(name, key string) error {
	s, done := r.g.hold()
	defer done()
	return s.Save(name, key)
}

func (r *Router) Clear(name string) error {
	s, done := r.g.hold()
	defer done()
	return s.Clear(name)
}

func (r *Router) Names() ([]string, error) {
	s, done := r.g.hold()
	defer done()
	return s.Names()
}

// ChatRouter is the credstore.Store the ChatGPT account is opened on, for the
// same reason: Replace points it at another store.
type ChatRouter struct{ g guarded[credstore.Store] }

var _ credstore.Store = (*ChatRouter)(nil)

// NewChatRouter is a ChatRouter over cur.
func NewChatRouter(cur credstore.Store) *ChatRouter {
	return &ChatRouter{g: guarded[credstore.Store]{cur: cur}}
}

// Replace is Router.Replace for the sign-in store.
func (r *ChatRouter) Replace(fn func(cur credstore.Store) (credstore.Store, error)) error {
	return r.g.replace(fn)
}

// The credstore.Store methods all go to the current store.
func (r *ChatRouter) Ensure() error {
	s, done := r.g.hold()
	defer done()
	return s.Ensure()
}

func (r *ChatRouter) Has() bool {
	s, done := r.g.hold()
	defer done()
	return s.Has()
}

func (r *ChatRouter) Location() string {
	s, done := r.g.hold()
	defer done()
	return s.Location()
}

func (r *ChatRouter) LoadRecord() ([]byte, error) {
	s, done := r.g.hold()
	defer done()
	return s.LoadRecord()
}

func (r *ChatRouter) SaveRecord(data []byte) error {
	s, done := r.g.hold()
	defer done()
	return s.SaveRecord(data)
}

func (r *ChatRouter) LoadHostID() (string, error) {
	s, done := r.g.hold()
	defer done()
	return s.LoadHostID()
}

func (r *ChatRouter) SaveHostID(id string) error {
	s, done := r.g.hold()
	defer done()
	return s.SaveHostID(id)
}

func (r *ChatRouter) TryLock() (func(), bool, error) {
	s, done := r.g.hold()
	defer done()
	return s.TryLock()
}
