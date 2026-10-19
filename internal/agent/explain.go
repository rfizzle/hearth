package agent

import (
	"errors"

	"github.com/rfizzle/llm/failure"
)

// providerNames is how a provider's id reads to a person.
var providerNames = map[string]string{
	"openai":     "OpenAI",
	"chatgpt":    "ChatGPT",
	"anthropic":  "Anthropic",
	"gemini":     "Gemini",
	"openrouter": "OpenRouter",
}

// Explain words a turn's error for the person who owns the bot: a cap or a
// stop in plain terms, and each provider failure class as what to do about
// it. An error it knows nothing about reads as itself.
func Explain(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrRoundCap):
		return "it used up its steps before it finished"
	case errors.Is(err, ErrWallCap):
		return "it ran out of time before it finished"
	case errors.Is(err, ErrCancelled):
		return "it was stopped"
	}
	f, ok := failure.As(err)
	if !ok {
		return err.Error()
	}
	who := "the model provider"
	if n, ok := providerNames[f.Provider]; ok {
		who = n
	} else if f.Provider != "" {
		who = f.Provider
	}
	switch f.Class {
	case failure.ClassAuth:
		return "the key was refused by " + who + "; sign in again or replace it"
	case failure.ClassNetwork:
		return "could not reach " + who
	case failure.ClassRateLimit:
		return who + " is limiting requests; try again in a moment"
	case failure.ClassOverloaded:
		return who + " is overloaded; try again in a moment"
	case failure.ClassQuota, failure.ClassUsageLimit:
		return "the model's limit was hit on " + who + "; the allowance is spent"
	case failure.ClassContextLength:
		return "the model's limit was hit: the conversation is too long for it"
	case failure.ClassModelNotFound:
		return who + " has no such model"
	case failure.ClassNotEligible:
		return who + " will not serve this account"
	case failure.ClassUnsupported:
		return who + " does not support what was asked"
	case failure.ClassMalformed:
		return who + " sent a reply that could not be read"
	}
	return "something went wrong talking to " + who
}
