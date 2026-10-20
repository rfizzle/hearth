package provider

import (
	"errors"
	"fmt"

	"github.com/rfizzle/llm/chatgpt/credstore"

	"github.com/rfizzle/hearth/internal/keystore"
)

// The credential hand-shake (D29). The app owns the Keychain and pushes it:
// a core that starts with no app keeps the owner-only file stores, and the
// first Sync of a launch moves the accounts onto a keystore.MemoryKeys loaded with the
// app's records. Whatever the files held that the app lacked comes back in
// the answer for the app to store; the files go only on a later Sync that
// holds every name they held, so a crash between the two calls loses
// nothing. From then on no credential is written to disk by any path.
// See docs/architecture.md#2-responsibilities

// errNoSync is why Sync and Export refuse accounts that were not made by Open.
var errNoSync = errors.New("these accounts have no files to sync with")

// Sync is the app's push of its Keychain records. The first call of a launch
// moves the accounts to memory, loaded with records and with whatever the
// core's own files held that records lacked, and returns those for the app to
// store. A later call whose records hold every name the files held (or a name
// the person has removed since) deletes the files; one that misses a name
// keeps them. Later records are not loaded: the memory store is the truth
// once it exists, and the app learns of each change from OnCredentialChange.
//
// An error names a credential, never holds a value.
func (a *Accounts) Sync(records []keystore.Credential) (toStore []keystore.Credential, err error) {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if a.keys == nil || a.store == nil {
		return nil, errNoSync
	}
	held := map[string]bool{}
	for _, c := range records {
		if err := keystore.CheckRecordName(c.Name); err != nil {
			return nil, err
		}
		if len(c.Value) == 0 {
			// An empty value would count as held and hide the file's copy.
			return nil, fmt.Errorf("the record %q has no value", c.Name)
		}
		held[c.Name] = true
	}
	if a.mem != nil {
		return nil, a.dropFiles(held)
	}
	mem := keystore.NewMemoryKeys()
	mem.OnChange = func(name string, deleted bool) {
		if f := a.OnCredentialChange; f != nil {
			f(name, deleted)
		}
	}
	// Read, preload and switch are one step under both routers' write
	// locks, so a save during the switch lands in memory, not in a file the
	// next sync deletes unread.
	err = a.keys.Replace(func(keystore.Store) (keystore.Store, error) {
		return mem, a.store.Replace(func(credstore.Store) (credstore.Store, error) {
			fromFiles, err := a.files().Read()
			if err != nil {
				return nil, err
			}
			if err := mem.Preload(records); err != nil {
				return nil, err
			}
			for _, c := range fromFiles {
				if !held[c.Name] {
					toStore = append(toStore, c)
				}
			}
			return mem, mem.Preload(toStore)
		})
	})
	if err != nil {
		return nil, err
	}
	a.mem = mem
	return toStore, nil
}

// Export is the record stored under name, for the app to put in its
// Keychain after a credential change. It answers keystore.ErrNoKey for a name that
// holds nothing, and before the first Sync, when the files are the store.
func (a *Accounts) Export(name string) (keystore.Credential, error) {
	a.syncMu.Lock()
	mem := a.mem
	a.syncMu.Unlock()
	if mem == nil {
		return keystore.Credential{}, keystore.ErrNoKey
	}
	if err := keystore.CheckRecordName(name); err != nil {
		return keystore.Credential{}, err
	}
	return mem.Export(name)
}

// files is the hand-run core's credential files as one thing.
func (a *Accounts) files() keystore.Files { return keystore.Files{Keys: a.fileKeys, Chat: a.fileChat} }

// dropFiles deletes keys/ and chatgpt/ once every name they hold is in held
// or was removed since the first Sync. One name missing keeps both.
func (a *Accounts) dropFiles(held map[string]bool) error {
	if a.filesGone {
		return nil
	}
	names, err := a.files().Names()
	if err != nil {
		return err
	}
	for _, n := range names {
		if !held[n] && !a.mem.WasRemoved(n) {
			return nil
		}
	}
	if err := a.files().Remove(); err != nil {
		return err
	}
	a.filesGone = true
	return nil
}
