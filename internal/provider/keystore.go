package provider

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// ErrNoKey is what Load returns for a name that holds nothing.
var ErrNoKey = errors.New("no key is stored under that name")

// KeyStore keeps secrets by name. A name is a provider's ("anthropic") or an
// integration's ("brave-search"), so one store serves both. The file store
// below is the stopgap; the Keychain-backed store implements the same
// interface later.
type KeyStore interface {
	// Load returns the secret, or ErrNoKey.
	Load(name string) (string, error)
	// Save stores the secret, replacing any earlier one.
	Save(name, key string) error
	// Clear removes the secret. Clearing a name that holds nothing is not an
	// error.
	Clear(name string) error
	// Names lists every name that holds a secret, so the scrub covers keys
	// whoever owns them.
	Names() ([]string, error)
}

// nameRe is the shape of a key name. It is also the file name, so it can
// never name a path.
var nameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

func checkName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("%q is not a valid key name", name)
	}
	return nil
}

// FileKeys is the file KeyStore: one 0600 file per name in a 0700 directory.
type FileKeys struct{ Dir string }

func (f FileKeys) path(name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	return filepath.Join(f.Dir, name), nil
}

// Load reads the secret stored under name.
func (f FileKeys) Load(name string) (string, error) {
	p, err := f.path(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNoKey
	}
	if err != nil {
		return "", fmt.Errorf("read the stored key: %w", err)
	}
	return string(data), nil
}

// Save writes the secret through a temporary file and a rename, so a crash
// never leaves half a key and the file is never readable by anyone else.
func (f FileKeys) Save(name, key string) error {
	p, err := f.path(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(f.Dir, 0o700); err != nil {
		return fmt.Errorf("create the key folder: %w", err)
	}
	if err := os.Chmod(f.Dir, 0o700); err != nil {
		return fmt.Errorf("protect the key folder: %w", err)
	}
	tmp, err := os.CreateTemp(f.Dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("store the key: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("store the key: %w", err)
	}
	if _, err := tmp.WriteString(key); err != nil {
		tmp.Close()
		return fmt.Errorf("store the key: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("store the key: %w", err)
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		return fmt.Errorf("store the key: %w", err)
	}
	return nil
}

// Clear removes the secret stored under name.
func (f FileKeys) Clear(name string) error {
	p, err := f.path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove the stored key: %w", err)
	}
	return nil
}

// Names lists the stored names in order.
func (f FileKeys) Names() ([]string, error) {
	entries, err := os.ReadDir(f.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list the stored keys: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && nameRe.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
