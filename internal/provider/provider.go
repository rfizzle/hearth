package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/chatgpt"
	"github.com/rfizzle/llm/chatgpt/credstore"

	// The plan route is a mode of the Responses backend; importing it makes
	// "chatgpt" resolvable.
	_ "github.com/rfizzle/llm/openai/responses"
)

// SignInCommand is what a signed-out message tells the person to run.
const SignInCommand = "hearthd login chatgpt"

// Init names hearth to llm. The helper calls it once at startup.
func Init() {
	llm.SetApp(llm.App{
		Name:       "hearth",
		EnvPrefix:  "HEARTH",
		SignInHint: "Settings → Accounts → Sign in with ChatGPT",
		URL:        "https://github.com/rfizzle/hearth",
	})
}

// DefaultDir is where the sign-in is kept: the application support folder of
// the person on a Mac.
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Hearth", "chatgpt"), nil
}

// Open opens the ChatGPT account kept in dir. client reaches the
// authorization server and nil is the real one.
func Open(dir string, client *http.Client) (*chatgpt.Account, error) {
	return chatgpt.Open(chatgpt.Options{Store: credstore.FileStore{Dir: dir}, Client: client})
}

// OpenBrowser opens the sign-in page in the default browser. It refuses any
// other address, so it cannot be turned into a way to launch a URL.
func OpenBrowser(page string) error {
	if !strings.HasPrefix(page, chatgpt.AuthorizeURL+"?") {
		return errors.New("refusing to open anything but the ChatGPT sign-in page")
	}
	return exec.Command("open", page).Run()
}

// Login runs the browser sign-in and tells the person who signed in on
// o.Say, their terminal.
func Login(ctx context.Context, acct *chatgpt.Account, o chatgpt.LoginOptions) error {
	res, err := acct.Login(ctx, o)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(o.Say, "Signed in to ChatGPT as %s.\n", res.Email)
	return err
}

// Logout signs the person out and reports whether the server confirmed it.
func Logout(ctx context.Context, acct *chatgpt.Account) (revoked bool, err error) {
	return acct.Logout(ctx)
}

// Resolve builds the ChatGPT plan provider from a stored sign-in. A person
// who is not signed in gets an error that Explain words.
func Resolve(acct *chatgpt.Account) (llm.Provider, error) {
	return llm.Resolve("chatgpt", llm.ResolveOpts{TokenSource: acct})
}

// Say sends one user message and writes the streamed reply to w.
func Say(ctx context.Context, p llm.Provider, text string, w io.Writer) error {
	events, err := p.StreamCompletion(ctx, []llm.Message{{Role: llm.RoleUser, Content: text}}, llm.CompletionOpts{})
	if err != nil {
		return err
	}
	for ev := range events {
		if ev.Err != nil {
			return ev.Err
		}
		if _, err := io.WriteString(w, ev.Token); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, "\n")
	return err
}

// Explain words an error for the person in one line. A signed-out account
// names the command that fixes it.
func Explain(err error) string {
	if errors.Is(err, llm.ErrChatGPTSignedOut) || errors.Is(err, chatgpt.ErrNotSignedIn) {
		return "Not signed in to ChatGPT. Run `" + SignInCommand + "`."
	}
	return err.Error()
}
