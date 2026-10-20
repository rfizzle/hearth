package provider

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/rfizzle/hearth/internal/keystore"
	"github.com/rfizzle/hearth/internal/provider/gateway"
	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/compat"
)

// Compatible is llm's name for an OpenAI-compatible service. Every service
// the person adds by address is one, told apart by its profile.
const Compatible Provider = "openai-compatible"

// ErrNoProfile is what a call about a service that is not stored returns.
var ErrNoProfile = errors.New("that service is not added, or was removed")

// Profile is one OpenAI-compatible service the person added: a company
// gateway, a hosted service such as Groq, or a runtime on this Mac. The key
// is kept beside it, never in it.
type Profile struct {
	// ID is minted when the service is added and never changes.
	ID           string
	Name         string
	BaseURL      string
	DefaultModel string
}

// profileFile is what is stored beside the key: everything but the key.
type profileFile struct {
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	DefaultModel string `json:"default_model"`
	// Added orders the profiles, so "the first one added" survives a
	// relaunch.
	Added int64 `json:"added"`
}

// A profile's key is stored under compat-<id> and its settings under
// compat-<id>-profile, in the same key store as every other secret. The id
// is hex, so the two names cannot be confused.
const (
	profilePrefix = "compat-"
	profileSuffix = "-profile"
)

func profileKeyName(id string) string  { return profilePrefix + id }
func profileMetaName(id string) string { return profilePrefix + id + profileSuffix }

// isProfileMeta reports whether a stored name holds a profile's settings
// rather than a secret.
func isProfileMeta(name string) bool {
	return strings.HasPrefix(name, profilePrefix) && strings.HasSuffix(name, profileSuffix)
}

// Profiles lists the services the person added, the first added first.
func (a *Accounts) Profiles() []Profile {
	names, _ := a.Keys.Names()
	type row struct {
		p     Profile
		added int64
	}
	var rows []row
	for _, n := range names {
		if !isProfileMeta(n) {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(n, profilePrefix), profileSuffix)
		f, err := a.loadProfile(id)
		if err != nil {
			continue
		}
		rows = append(rows, row{Profile{ID: id, Name: f.Name, BaseURL: f.BaseURL, DefaultModel: f.DefaultModel}, f.Added})
	}
	slices.SortStableFunc(rows, func(x, y row) int { return cmp.Or(cmp.Compare(x.added, y.added), cmp.Compare(x.p.ID, y.p.ID)) })
	out := make([]Profile, len(rows))
	for i, r := range rows {
		out[i] = r.p
	}
	return out
}

func (a *Accounts) loadProfile(id string) (profileFile, error) {
	if err := keystore.CheckName(profileMetaName(id)); err != nil {
		return profileFile{}, ErrNoProfile
	}
	raw, err := a.Keys.Load(profileMetaName(id))
	if err != nil {
		return profileFile{}, err
	}
	var f profileFile
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		return profileFile{}, fmt.Errorf("read the settings of a service: %w", err)
	}
	return f, nil
}

// profileAccount is what List says about one profile: signed in, since a
// profile exists only once it is saved, with the key's last four
// characters and never the key.
func (a *Accounts) profileAccount(p Profile) Account {
	acct := Account{Provider: Compatible, State: SignedIn, Profile: p.ID, DisplayName: p.Name, BaseURL: p.BaseURL, DefaultModel: p.DefaultModel}
	if key, err := a.Keys.Load(profileKeyName(p.ID)); err == nil {
		acct.Label = lastFour(key)
	}
	return acct
}

// SetProfile adds a service when p.ID is empty, minting its id, or changes
// the one p.ID names. A change with an empty key keeps the stored key, unless
// the address moved to another host. The
// key is required unless the service runs on this Mac, and a key is never
// sent to another machine over plain http. It returns the account as List
// reports it, and publishes it.
// See docs/model.md#providers-and-models
func (a *Accounts) SetProfile(p Profile, key string) (Account, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.DefaultModel = strings.TrimSpace(p.DefaultModel)
	key = strings.TrimSpace(key)
	base, local, err := gateway.CheckBaseURL(p.BaseURL)
	if err != nil {
		return Account{}, err
	}
	p.BaseURL = base
	switch {
	case p.Name == "":
		return Account{}, errors.New("give the service a name, the field was empty")
	case p.DefaultModel == "":
		return Account{}, errors.New("name the model to use, the field was empty")
	}

	a.profileMu.Lock()
	defer a.profileMu.Unlock()
	f := profileFile{Name: p.Name, BaseURL: p.BaseURL, DefaultModel: p.DefaultModel, Added: time.Now().UnixNano()}
	adding := p.ID == ""
	if adding {
		if p.ID, err = a.mintProfileID(); err != nil {
			return Account{}, err
		}
	} else {
		old, err := a.loadProfile(p.ID)
		if errors.Is(err, keystore.ErrNoKey) {
			return Account{}, ErrNoProfile
		}
		if err != nil {
			return Account{}, err
		}
		f.Added = old.Added
		if key == "" && !local && gateway.HostMoved(old.BaseURL, p.BaseURL) {
			return Account{}, errors.New("the address changed; paste the key again")
		}
	}
	if key == "" && !local {
		if _, err := a.Keys.Load(profileKeyName(p.ID)); err != nil {
			return Account{}, errors.New("paste the key first, the field was empty")
		}
	}
	raw, err := json.Marshal(f)
	if err != nil {
		return Account{}, err
	}
	// The settings are what make a service exist, so they are written after
	// the key and removed before it: a reader never sees a service without
	// its key, and a failure part way leaves at most a key nobody lists,
	// which the scrub still covers.
	if key != "" {
		if err := a.Keys.Save(profileKeyName(p.ID), key); err != nil {
			return Account{}, err
		}
	}
	if err := a.Keys.Save(profileMetaName(p.ID), string(raw)); err != nil {
		if adding {
			_ = a.Keys.Clear(profileKeyName(p.ID))
		}
		return Account{}, err
	}
	acct := a.profileAccount(p)
	if a.OnChange != nil {
		a.OnChange(acct)
	}
	return acct, nil
}

// ClearProfile removes a service and its key, and publishes it signed out
// so whoever shows it drops its card.
func (a *Accounts) ClearProfile(id string) error {
	a.profileMu.Lock()
	defer a.profileMu.Unlock()
	f, err := a.loadProfile(id)
	if errors.Is(err, keystore.ErrNoKey) {
		return ErrNoProfile
	}
	if err != nil {
		return err
	}
	if err := a.Keys.Clear(profileMetaName(id)); err != nil {
		return err
	}
	if err := a.Keys.Clear(profileKeyName(id)); err != nil {
		return err
	}
	if a.OnChange != nil {
		a.OnChange(Account{Provider: Compatible, State: SignedOut, Profile: id, DisplayName: f.Name, BaseURL: f.BaseURL, DefaultModel: f.DefaultModel})
	}
	return nil
}

// mintProfileID is a fresh id: eight hex digits no stored profile uses.
func (a *Accounts) mintProfileID() (string, error) {
	for range 8 {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		id := hex.EncodeToString(b)
		if _, err := a.Keys.Load(profileMetaName(id)); errors.Is(err, keystore.ErrNoKey) {
			return id, nil
		}
	}
	return "", errors.New("could not find a free id for the service")
}

// buildProfile makes the provider for one profile through llm's compatible
// backend, under the profile's name so a failure says which service. It
// sends through the accounts' client, nil being a plain one.
func (a *Accounts) buildProfile(p Profile) (llm.Provider, error) {
	key, err := a.Keys.Load(profileKeyName(p.ID))
	if err != nil && !errors.Is(err, keystore.ErrNoKey) {
		return nil, err
	}
	cfg := openai.DefaultConfig(key)
	cfg.BaseURL = p.BaseURL
	cfg.HTTPClient = gateway.Peeking(a.client)
	prov := compat.NewNamed(openai.NewClientWithConfig(cfg), p.DefaultModel, p.BaseURL, p.Name)
	return gateway.Check(prov, p.Name, func(s string) string { return a.vault().ScrubMessage(llm.Message{Content: s}).Content }), nil
}
