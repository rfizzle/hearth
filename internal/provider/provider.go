package provider

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/chatgpt"

	// Each backend registers itself by name when imported. The plan route is a
	// mode of the Responses backend, so importing it makes "chatgpt"
	// resolvable. Anthropic and Gemini are not linked yet: their SDKs are new
	// module requirements, so SetKey refuses them in words.
	_ "github.com/rfizzle/llm/openai/responses"
	_ "github.com/rfizzle/llm/openrouter"
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

// DefaultDir is where the core keeps its data: the application support folder
// of the person on a Mac.
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Hearth"), nil
}

// OpenBrowser opens the sign-in page in the default browser. It refuses any
// other address, so it cannot be turned into a way to launch a URL.
func OpenBrowser(page string) error {
	if !strings.HasPrefix(page, chatgpt.AuthorizeURL+"?") {
		return errors.New("refusing to open anything but the ChatGPT sign-in page")
	}
	return exec.Command("open", page).Run()
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
	if errors.Is(err, ErrNoAccount) || errors.Is(err, llm.ErrChatGPTSignedOut) || errors.Is(err, chatgpt.ErrNotSignedIn) {
		return "Not signed in to ChatGPT. Run `" + SignInCommand + "`."
	}
	return err.Error()
}
