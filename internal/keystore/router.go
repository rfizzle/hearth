package keystore

import (
	"sync"

	"github.com/rfizzle/llm/chatgpt/credstore"
)

// Router is the Store every holder of a shared key store keeps. Use points it
// at another store, so the holders (the integrations, say) need not be told
// when the core moves from its files to memory.
type Router struct {
	mu  sync.RWMutex
	cur Store
}

// NewRouter is a Router over cur.
func NewRouter(cur Store) *Router { return &Router{cur: cur} }

// Use makes s the store every call reaches from now on.
func (r *Router) Use(s Store) {
	r.mu.Lock()
	r.cur = s
	r.mu.Unlock()
}

func (r *Router) now() Store {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cur
}

// Load, Save, Clear and Names go to the current store.
func (r *Router) Load(name string) (string, error) { return r.now().Load(name) }
func (r *Router) Save(name, key string) error      { return r.now().Save(name, key) }
func (r *Router) Clear(name string) error          { return r.now().Clear(name) }
func (r *Router) Names() ([]string, error)         { return r.now().Names() }

// ChatRouter is the credstore.Store the ChatGPT account is opened on, for the
// same reason: Use points it at another store.
type ChatRouter struct {
	mu  sync.RWMutex
	cur credstore.Store
}

var _ credstore.Store = (*ChatRouter)(nil)

// NewChatRouter is a ChatRouter over cur.
func NewChatRouter(cur credstore.Store) *ChatRouter { return &ChatRouter{cur: cur} }

// Use makes s the store every call reaches from now on.
func (r *ChatRouter) Use(s credstore.Store) {
	r.mu.Lock()
	r.cur = s
	r.mu.Unlock()
}

func (r *ChatRouter) now() credstore.Store {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cur
}

// The credstore.Store methods all go to the current store.
func (r *ChatRouter) Ensure() error                  { return r.now().Ensure() }
func (r *ChatRouter) Has() bool                      { return r.now().Has() }
func (r *ChatRouter) Location() string               { return r.now().Location() }
func (r *ChatRouter) LoadRecord() ([]byte, error)    { return r.now().LoadRecord() }
func (r *ChatRouter) SaveRecord(data []byte) error   { return r.now().SaveRecord(data) }
func (r *ChatRouter) LoadHostID() (string, error)    { return r.now().LoadHostID() }
func (r *ChatRouter) SaveHostID(id string) error     { return r.now().SaveHostID(id) }
func (r *ChatRouter) TryLock() (func(), bool, error) { return r.now().TryLock() }
