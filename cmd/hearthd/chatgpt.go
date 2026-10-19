package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/llm/chatgpt"
)

const chatgptUsage = "usage: hearthd login chatgpt | hearthd logout chatgpt | hearthd say <message>"

// runLogin signs in to ChatGPT in the default browser.
func runLogin(args []string) int {
	return runAccount(args, func(ctx context.Context, dir string) error {
		acct, err := provider.Open(dir, nil)
		if err != nil {
			return err
		}
		return acct.Login(ctx, chatgpt.LoginOptions{Browser: provider.OpenBrowser, Say: os.Stdout})
	})
}

// runLogout signs out of ChatGPT.
func runLogout(args []string) int {
	return runAccount(args, func(ctx context.Context, dir string) error {
		acct, err := provider.Open(dir, nil)
		if err != nil {
			return err
		}
		revoked, err := acct.Logout(ctx)
		if err != nil {
			return err
		}
		if !revoked {
			fmt.Println("Signed out here; ChatGPT did not confirm, so disconnect hearth in your ChatGPT settings too.")
			return nil
		}
		fmt.Println("Signed out of ChatGPT.")
		return nil
	})
}

// runSay sends one message through the stored sign-in and prints the reply.
func runSay(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, chatgptUsage)
		return 2
	}
	return runAccount([]string{"chatgpt"}, func(ctx context.Context, dir string) error {
		acct, err := provider.Open(dir, nil)
		if err != nil {
			return err
		}
		m, err := acct.Resolve(provider.Balanced)
		if err != nil {
			return err
		}
		return provider.Say(ctx, m.Provider, strings.Join(args, " "), os.Stdout)
	})
}

// runAccount checks the "chatgpt" argument, names hearth to llm and runs fn
// against the sign-in folder, wording a failure for the person.
func runAccount(args []string, fn func(ctx context.Context, dir string) error) int {
	if len(args) != 1 || args[0] != "chatgpt" {
		fmt.Fprintln(os.Stderr, chatgptUsage)
		return 2
	}
	provider.Init()
	dir, err := provider.DefaultDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hearthd:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := fn(ctx, dir); err != nil {
		fmt.Fprintln(os.Stderr, provider.Explain(err))
		return 1
	}
	return 0
}
