package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rfizzle/hearth/internal/org"
)

// Tier says whether a tool's effect stays where the core can see it.
type Tier int

// The tiers. A read-tier tool runs when its grant allows it; an act-tier
// tool also needs the owner's approval for each call, so it has no standing.
const (
	TierRead Tier = iota
	TierAct
)

// Call is one checked call as an executor receives it. Path and Folder are
// set for a files capability only: Path is the requested path resolved to
// an absolute path with its symlinks followed, and Folder the granted folder
// it lies in, resolved the same way.
type Call struct {
	Args   json.RawMessage
	Path   string
	Folder string
}

// Result is what a call produced: Text goes back to the model, Files are
// the files it wrote, for the owner to open.
type Result struct {
	Text  string
	Files []org.FileRef
}

// Definition is one tool a model may call. Parameters is its JSON Schema;
// Describe turns a call's arguments into the plain words the owner sees in
// place of the JSON, and must not fail on arguments it cannot parse.
// NoStanding marks a tool the owner may allow once or for a conversation but
// never for good, such as deleting a file: every other call to it waits for
// them.
type Definition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
	Capability  org.Capability
	Tier        Tier
	NoStanding  bool
	Execute     func(ctx context.Context, call Call) (Result, error)
	Describe    func(args json.RawMessage) string
}

// Standing reports whether the owner may allow the tool for good: a
// read-tier tool not marked NoStanding.
// See docs/model.md#8-approvals.
func (d Definition) Standing() bool {
	return d.Tier == TierRead && !d.NoStanding
}

// Registry is a set of definitions in the order they were added. The zero
// value is empty and ready to use.
type Registry struct {
	defs []Definition
}

// NewRegistry is a registry holding defs, refusing two with the same name.
func NewRegistry(defs ...Definition) (*Registry, error) {
	r := &Registry{}
	for _, d := range defs {
		if err := r.Add(d); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Add appends d, refusing a definition with no name, no executor or a name
// already taken.
func (r *Registry) Add(d Definition) error {
	if d.Name == "" || d.Execute == nil {
		return fmt.Errorf("tools: definition %q has no name or no executor", d.Name)
	}
	if _, ok := r.Lookup(d.Name); ok {
		return fmt.Errorf("tools: %s is defined twice", d.Name)
	}
	r.defs = append(r.defs, d)
	return nil
}

// Lookup is the definition named name.
func (r *Registry) Lookup(name string) (Definition, bool) {
	if r == nil {
		return Definition{}, false
	}
	for _, d := range r.defs {
		if d.Name == name {
			return d, true
		}
	}
	return Definition{}, false
}

// All is every definition, in the order they were added.
func (r *Registry) All() []Definition {
	if r == nil {
		return nil
	}
	return append([]Definition(nil), r.defs...)
}
