package provider

import "context"

// Clear removes whatever credential p holds: the stored key for a provider
// that takes one, the sign-in for ChatGPT. It is the one call behind "remove
// this account" whichever kind the account is.
func (a *Accounts) Clear(ctx context.Context, p Provider) error {
	if p == ChatGPT {
		_, err := a.Logout(ctx)
		return err
	}
	return a.ClearKey(p)
}
