package org

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Org is the one organisation an install runs: its name and the folder its
// bots write to.
type Org struct {
	Name          string
	WorkspacePath string
}

// Tier is the model quality a bot asks for; the provider maps it to a model.
type Tier int

// The tiers, from cheapest to most capable.
const (
	TierFast     Tier = 1
	TierBalanced Tier = 2
	TierBest     Tier = 3
)

// BotStatus is whether a bot is free or busy.
type BotStatus int

// The statuses a bot can be in.
const (
	BotIdle    BotStatus = 1
	BotWorking BotStatus = 2
)

// Scope is where a bot works: on a team, or for the whole org, above the
// teams, as the Chief of Staff does.
// See docs/model.md#people
type Scope int

// The scopes a bot can have.
const (
	ScopeTeam Scope = 1
	ScopeOrg  Scope = 2
)

// Bot is one employee. FolderPath is its own folder in the workspace and
// DMConversationID its direct conversation with the owner.
type Bot struct {
	ID               string
	Name             string
	Role             string
	JobDescription   string
	Tier             Tier
	Status           BotStatus
	Avatar           string
	FolderPath       string
	DMConversationID string
	CreatedAt        time.Time

	// ModelOverride names a model that wins over the one the tier maps to;
	// empty when the bot follows its tier.
	ModelOverride string

	// Scope is a team's, or the org's for a bot above the teams.
	Scope Scope
}

// RoleTemplate is a starting point for hiring: a role, a summary of it, the
// job description a new bot gets, the tier it suggests and the scope the bot
// works at.
type RoleTemplate struct {
	ID             string
	Role           string
	Summary        string
	JobDescription string
	SuggestedTier  Tier
	Scope          Scope
}

// NewBot returns a new idle team bot, the grants a bot starts with and its DM with
// the owner. The bot's folder is its file-system-safe name under
// workspaceRoot. A new bot can read and write its own folder, fetch pages,
// search the web and ask the owner a question, and must ask before deleting
// a file in its folder; it is granted nothing else.
// See docs/model.md#access.
func NewBot(name, role, desc string, tier Tier, workspaceRoot string) (Bot, []Grant, Conversation) {
	folder := filepath.Join(workspaceRoot, FolderName(name))
	bot := Bot{
		Name:           name,
		Role:           role,
		JobDescription: desc,
		Tier:           tier,
		Status:         BotIdle,
		FolderPath:     folder,
		Scope:          ScopeTeam,
	}
	grants := []Grant{
		{Capability: FilesRead, Mode: ModeAllowed, Folder: folder},
		{Capability: FilesWrite, Mode: ModeAllowed, Folder: folder},
		{Capability: FilesDelete, Mode: ModeAsk, Folder: folder},
		{Capability: WebFetch, Mode: ModeAllowed},
		{Capability: WebSearch, Mode: ModeAllowed},
		{Capability: AskOwner, Mode: ModeAllowed},
		{Capability: Remember, Mode: ModeAllowed},
	}
	dm := Conversation{Kind: ConversationDM, Title: name}
	return bot, grants, dm
}

// InboxFolder is the name of the folder in the Hearth folder that dropped
// files land in. Every bot may read it, so no bot may own it.
const InboxFolder = "Inbox"

// ErrReservedName refuses a bot whose folder would be the Hearth folder's
// Inbox.
var ErrReservedName = errors.New("that name is taken by the Hearth folder's Inbox, where dropped files go; choose another")

// CheckName is ErrReservedName for a bot name whose folder would be the
// Inbox, in any case, since the Mac's disk does not tell the two apart.
// See docs/model.md#access.
func CheckName(name string) error {
	if strings.EqualFold(FolderName(name), InboxFolder) {
		return fmt.Errorf("%q: %w", name, ErrReservedName)
	}
	return nil
}

// FolderName is the form of a bot's name its folder is created under: path
// separators, colons and control characters become hyphens, and surrounding
// spaces and leading dots go, so the folder is one visible path element. A
// name with nothing left becomes "Bot".
func FolderName(name string) string {
	clean := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || unicode.IsControl(r) {
			return '-'
		}
		return r
	}, name)
	clean = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(clean), "."))
	if clean == "" {
		return "Bot"
	}
	return clean
}
