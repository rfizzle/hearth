package org

// Capability names one thing a bot may do, as the owner sees it in the
// access table.
type Capability string

// The capabilities the core tools check.
const (
	FilesRead   Capability = "files.read"
	FilesWrite  Capability = "files.write"
	FilesDelete Capability = "files.delete"
	WebFetch    Capability = "web.fetch"
	WebSearch   Capability = "web.search"
	AskOwner    Capability = "ask_owner"
)

// Mode is how a capability is granted. The zero value is not granted.
type Mode int

// The modes, from least to most permissive.
const (
	ModeNotGranted Mode = 0
	ModeAsk        Mode = 1
	ModeAllowed    Mode = 2
)

// Grant gives one bot one capability in one mode. Folder scopes the files
// capabilities and is empty for the rest.
type Grant struct {
	BotID      string
	Capability Capability
	Mode       Mode
	Folder     string
}

// Allowed is the most permissive mode any grant gives capability, and
// ModeNotGranted when none names it.
func Allowed(grants []Grant, capability Capability) Mode {
	mode := ModeNotGranted
	for _, g := range grants {
		if g.Capability == capability && g.Mode > mode {
			mode = g.Mode
		}
	}
	return mode
}

// FolderGrants is the folders capability is allowed in without asking. A
// folder granted only in ask mode is left out, so a caller with no approval
// gate refuses it rather than acting.
func FolderGrants(grants []Grant, capability Capability) []string {
	return Folders(grants, capability, ModeAllowed)
}

// Folders is the folders capability is granted in at atLeast or a more
// permissive mode, in the order the grants are given. A grant with no folder
// names none.
func Folders(grants []Grant, capability Capability, atLeast Mode) []string {
	var folders []string
	for _, g := range grants {
		if g.Capability == capability && g.Mode >= atLeast && g.Folder != "" {
			folders = append(folders, g.Folder)
		}
	}
	return folders
}
