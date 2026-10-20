package keystore

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/rfizzle/llm/chatgpt/credstore"
)

// Files is the whole of a hand-run core's credential files: the keys folder
// and the ChatGPT sign-in folder. The app sync reads them once, hands what the
// app lacks back to it, and deletes them when the app holds everything.
type Files struct {
	Keys FileKeys
	Chat credstore.FileStore
}

// Read is every record the files hold. It reads values and returns them to
// the sync only.
func (f Files) Read() ([]Credential, error) {
	names, err := f.Keys.Names()
	if err != nil {
		return nil, err
	}
	var out []Credential
	for _, n := range names {
		if Reserved(n) {
			return nil, fmt.Errorf("the key folder holds a key named %q, which hearth keeps for the ChatGPT sign-in; move it away", n)
		}
		v, err := f.Keys.Load(n)
		if err != nil {
			return nil, err
		}
		out = append(out, Credential{Name: n, Value: []byte(v)})
	}
	rec, err := f.Chat.LoadRecord()
	switch {
	case err == nil:
		out = append(out, Credential{Name: ChatRecord, Value: rec})
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("read the ChatGPT sign-in: %w", err)
	}
	host, err := f.Chat.LoadHostID()
	switch {
	case err == nil:
		out = append(out, Credential{Name: ChatHost, Value: []byte(host)})
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("read the ChatGPT host id: %w", err)
	}
	return out, nil
}

// Names is the names the files hold now, without keeping a value.
func (f Files) Names() ([]string, error) {
	names, err := f.Keys.Names()
	if err != nil {
		return nil, err
	}
	if f.Chat.Has() {
		names = append(names, ChatRecord)
	}
	if _, err := f.Chat.LoadHostID(); err == nil {
		names = append(names, ChatHost)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read the ChatGPT host id: %w", err)
	}
	return names, nil
}

// Remove deletes both folders.
func (f Files) Remove() error {
	for _, d := range []string{f.Keys.Dir, f.Chat.Dir} {
		if err := os.RemoveAll(d); err != nil {
			return fmt.Errorf("remove the old credential files: %w", err)
		}
	}
	return nil
}
