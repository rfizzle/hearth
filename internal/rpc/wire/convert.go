package wire

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rfizzle/hearth/internal/keystore"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/provider"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// The conversions more than one service needs. An enum that org and the
// contract number alike is a cast, and a test holds the two lists together;
// one that differs in kind (a provider's name) goes through a table.

// Timestamp is t in the contract's form; nil for the zero time.
func Timestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// TierFromProto is the tier the contract names, or invalid_argument for
// none.
func TierFromProto(t hearthv1.Tier) (org.Tier, error) {
	switch t {
	case hearthv1.Tier_TIER_FAST, hearthv1.Tier_TIER_BALANCED, hearthv1.Tier_TIER_BEST:
		return org.Tier(t), nil
	}
	return 0, Invalid("tier is required")
}

// BotToProto is a bot as the app shows it.
func BotToProto(b org.Bot) *hearthv1.Bot {
	return &hearthv1.Bot{
		Id:               b.ID,
		Name:             b.Name,
		Role:             b.Role,
		JobDescription:   b.JobDescription,
		Tier:             hearthv1.Tier(b.Tier),
		Status:           hearthv1.BotStatus(b.Status),
		Avatar:           b.Avatar,
		FolderPath:       b.FolderPath,
		DmConversationId: b.DMConversationID,
		ModelOverride:    b.ModelOverride,
		Scope:            hearthv1.BotScope(b.Scope),
	}
}

// ProviderNames is the contract's provider enum against provider's names.
var ProviderNames = map[hearthv1.Provider]provider.Provider{
	hearthv1.Provider_PROVIDER_CHATGPT:           provider.ChatGPT,
	hearthv1.Provider_PROVIDER_ANTHROPIC:         provider.Anthropic,
	hearthv1.Provider_PROVIDER_GEMINI:            provider.Gemini,
	hearthv1.Provider_PROVIDER_OPENROUTER:        provider.OpenRouter,
	hearthv1.Provider_PROVIDER_OPENAI_COMPATIBLE: provider.Compatible,
}

// FromProto is the domain name for a contract enum, or invalid_argument when
// hearth has none for it (unspecified, or not built yet).
func FromProto[E interface {
	comparable
	fmt.Stringer
}, N any](names map[E]N, e E) (N, error) {
	n, ok := names[e]
	if !ok {
		return n, Invalid("%s is not one hearth can use", e)
	}
	return n, nil
}

// ToProto is the contract enum for a domain name; the zero value, which the
// contract reads as unspecified, when there is none.
func ToProto[E, N comparable](names map[E]N, n N) E {
	for e, name := range names {
		if name == n {
			return e
		}
	}
	var none E
	return none
}

// CredentialToProto is a record with its value. Only the sync and export
// replies call it; no event body is built from it.
func CredentialToProto(c keystore.Credential) *hearthv1.Credential {
	return &hearthv1.Credential{Name: c.Name, Value: c.Value}
}

// CredentialsFromProto is the records the app sent from its Keychain.
func CredentialsFromProto(in []*hearthv1.Credential) []keystore.Credential {
	return Each(in, func(r *hearthv1.Credential) keystore.Credential {
		return keystore.Credential{Name: r.GetName(), Value: r.GetValue()}
	})
}
