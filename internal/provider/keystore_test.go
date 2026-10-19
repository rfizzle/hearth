package provider

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFileKeys(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "keys")
	k := FileKeys{Dir: dir}

	if _, err := k.Load("anthropic"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("Load of nothing = %v, want ErrNoKey", err)
	}
	if names, err := k.Names(); err != nil || len(names) != 0 {
		t.Fatalf("Names of nothing = %v, %v", names, err)
	}
	if err := k.Save("brave-search", "first"); err != nil {
		t.Fatal(err)
	}
	if err := k.Save("anthropic", "sk-1"); err != nil {
		t.Fatal(err)
	}
	if err := k.Save("anthropic", "sk-2"); err != nil {
		t.Fatal(err)
	}
	if got, err := k.Load("anthropic"); err != nil || got != "sk-2" {
		t.Errorf("Load = %q, %v; want the replacement", got, err)
	}
	if names, _ := k.Names(); !slices.Equal(names, []string{"anthropic", "brave-search"}) {
		t.Errorf("Names = %v", names)
	}

	// The folder is the owner's alone, and so is each file; no temporary
	// file is left behind.
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, %v; want 0700", info.Mode().Perm(), err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("folder holds %d entries, want 2", len(entries))
	}
	for _, e := range entries {
		if fi, err := e.Info(); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %v, %v; want 0600", e.Name(), fi.Mode().Perm(), err)
		}
	}

	if err := k.Clear("anthropic"); err != nil {
		t.Fatal(err)
	}
	if err := k.Clear("anthropic"); err != nil {
		t.Errorf("Clear of nothing = %v, want nil", err)
	}
	if _, err := k.Load("anthropic"); !errors.Is(err, ErrNoKey) {
		t.Errorf("Load after Clear = %v", err)
	}
}

func TestFileKeysRefuseNamesThatAreNotNames(t *testing.T) {
	dir := t.TempDir()
	k := FileKeys{Dir: filepath.Join(dir, "keys")}
	for _, name := range []string{"", "../escape", "a/b", ".hidden", "UPPER", "with space", "9lives"} {
		if err := k.Save(name, "x"); err == nil {
			t.Errorf("Save(%q) succeeded", name)
		}
		if _, err := k.Load(name); err == nil || errors.Is(err, ErrNoKey) {
			t.Errorf("Load(%q) = %v, want a refusal", name, err)
		}
		if err := k.Clear(name); err == nil {
			t.Errorf("Clear(%q) succeeded", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "escape")); err == nil {
		t.Error("a name wrote outside the folder")
	}
}
