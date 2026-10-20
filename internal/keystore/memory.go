package keystore

import (
	"fmt"
	"io/fs"
	"sort"
	"sync"

	"github.com/rfizzle/llm/chatgpt/credstore"
)

// The names the ChatGPT sign-in is kept under beside the key names, so the
// app's Keychain holds every credential as one flat list of records.
const (
	ChatRecord = "chatgpt"
	ChatHost   = "chatgpt-host"
)

// Reserved reports whether name is one the ChatGPT sign-in owns.
func Reserved(name string) bool { return name == ChatRecord || name == ChatHost }

// Credential is one named record the app keeps in its Keychain: an API key,
// an integration's key, a service's settings, the ChatGPT sign-in record or
// the ChatGPT host id. Value is the secret itself; it leaves the core only
// in the answer to Sync and Export, and nothing prints it.
type Credential struct {
	Name  string
	Value []byte
}

// MemoryKeys holds every credential in memory and writes none of them
// anywhere. It is the Store for the keys and the credstore.Store for the
// ChatGPT sign-in, so the one object is the core's whole credential state
// once the app has pushed its Keychain records (D29). Nothing in it is ever
// logged, formatted into an error or sent on the event bus; OnChange gets a
// name and a flag, never a value.
// See docs/architecture.md#2-responsibilities
type MemoryKeys struct {
	// OnChange is told the name of each credential that was stored, replaced
	// or removed after Preload, and whether it was removed. Set it before the
	// store is shared; it runs on the goroutine that made the change, with no
	// lock held, and must not block.
	OnChange func(name string, deleted bool)

	mu      sync.Mutex
	keys    map[string]string
	record  []byte // the ChatGPT record, nil when none
	hostID  string
	hasHost bool
	// removed holds the names taken away since Preload, so the file cleanup
	// can tell a name the person removed from one the app never received.
	removed map[string]bool

	lock sync.Mutex // the credstore lock; one process is behind this store
}

var (
	_ Store           = (*MemoryKeys)(nil)
	_ credstore.Store = (*MemoryKeys)(nil)
)

// NewMemoryKeys is an empty store.
func NewMemoryKeys() *MemoryKeys {
	return &MemoryKeys{keys: map[string]string{}, removed: map[string]bool{}}
}

// CheckRecordName accepts a key name or one of the sign-in's two names. The
// error never holds a value.
func CheckRecordName(name string) error {
	if Reserved(name) {
		return nil
	}
	return CheckName(name)
}

// Preload puts records in without telling OnChange: they came from the app
// or from the core's own files, and the caller says which to the app.
func (m *MemoryKeys) Preload(records []Credential) error {
	for _, c := range records {
		if err := CheckRecordName(c.Name); err != nil {
			return err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range records {
		m.set(c.Name, c.Value)
	}
	return nil
}

// set stores one record. The caller holds mu.
func (m *MemoryKeys) set(name string, value []byte) {
	delete(m.removed, name)
	switch name {
	case ChatRecord:
		m.record = append([]byte(nil), value...)
	case ChatHost:
		m.hostID, m.hasHost = string(value), true
	default:
		m.keys[name] = string(value)
	}
}

func (m *MemoryKeys) changed(name string, deleted bool) {
	if m.OnChange != nil {
		m.OnChange(name, deleted)
	}
}

// Export is the record stored under name, or ErrNoKey. With Sync's
// records_to_store it is the only way a value leaves the store for the rpc
// layer.
func (m *MemoryKeys) Export(name string) (Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch name {
	case ChatRecord:
		if m.record != nil {
			return Credential{Name: name, Value: append([]byte(nil), m.record...)}, nil
		}
	case ChatHost:
		if m.hasHost {
			return Credential{Name: name, Value: []byte(m.hostID)}, nil
		}
	default:
		if v, ok := m.keys[name]; ok {
			return Credential{Name: name, Value: []byte(v)}, nil
		}
	}
	return Credential{}, ErrNoKey
}

// WasRemoved reports whether name was taken away since Preload.
func (m *MemoryKeys) WasRemoved(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.removed[name]
}

func errReserved(name string) error {
	return fmt.Errorf("%q is a name the ChatGPT sign-in keeps for itself", name)
}

// Store.

// Load returns the key stored under name, or ErrNoKey.
func (m *MemoryKeys) Load(name string) (string, error) {
	if err := CheckName(name); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.keys[name]
	if !ok {
		return "", ErrNoKey
	}
	return v, nil
}

// Save stores a key, replacing any earlier one.
func (m *MemoryKeys) Save(name, key string) error {
	if err := CheckName(name); err != nil {
		return err
	}
	if Reserved(name) {
		return errReserved(name)
	}
	m.mu.Lock()
	m.set(name, []byte(key))
	m.mu.Unlock()
	m.changed(name, false)
	return nil
}

// Clear removes a key. A name that holds nothing is not an error and is not
// a change.
func (m *MemoryKeys) Clear(name string) error {
	if err := CheckName(name); err != nil {
		return err
	}
	m.mu.Lock()
	_, had := m.keys[name]
	delete(m.keys, name)
	m.removed[name] = true
	m.mu.Unlock()
	if had {
		m.changed(name, true)
	}
	return nil
}

// Names lists the key names, not the sign-in's two records.
func (m *MemoryKeys) Names() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.keys))
	for n := range m.keys {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

// credstore.Store, for the ChatGPT sign-in.

// Ensure has nothing to prepare.
func (m *MemoryKeys) Ensure() error { return nil }

// Has reports whether a sign-in record is held.
func (m *MemoryKeys) Has() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.record != nil
}

// Location names where the record is, for a sentence in an error.
func (m *MemoryKeys) Location() string { return "the app's Keychain" }

// LoadRecord is the sign-in record, or fs.ErrNotExist.
func (m *MemoryKeys) LoadRecord() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.record == nil {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), m.record...), nil
}

// SaveRecord replaces the sign-in record and tells OnChange. A token refresh
// arrives here too, so the app learns of every new token.
func (m *MemoryKeys) SaveRecord(data []byte) error {
	m.mu.Lock()
	m.set(ChatRecord, data)
	m.mu.Unlock()
	m.changed(ChatRecord, false)
	return nil
}

// LoadHostID is the host id, or fs.ErrNotExist.
func (m *MemoryKeys) LoadHostID() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.hasHost {
		return "", fs.ErrNotExist
	}
	return m.hostID, nil
}

// SaveHostID stores the host id and tells OnChange.
func (m *MemoryKeys) SaveHostID(id string) error {
	m.mu.Lock()
	m.set(ChatHost, []byte(id))
	m.mu.Unlock()
	m.changed(ChatHost, false)
	return nil
}

// TryLock takes the lock if nobody holds it: one process is behind this
// store, so a mutex excludes every other Account that shares the record.
func (m *MemoryKeys) TryLock() (func(), bool, error) {
	if !m.lock.TryLock() {
		return nil, false, nil
	}
	return m.lock.Unlock, true, nil
}
