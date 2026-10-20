package skill

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rfizzle/hearth/internal/org/intro"
)

// introductionOf reads the introduction key of a template's frontmatter,
// which agentcore's parser passes over. The key is a mapping of three quoted
// strings: purpose, how (three lines, each a symbol name, a space and the
// words) and try (three prompts), the lines of the last two separated by \n.
func introductionOf(file string) (intro.Introduction, error) {
	file = strings.ReplaceAll(file, "\r\n", "\n")
	front, _, _ := strings.Cut(strings.TrimPrefix(file, "---\n"), "\n---\n")
	_, block, ok := strings.Cut("\n"+front, "\nintroduction:\n")
	if !ok {
		return intro.Introduction{}, fmt.Errorf("no introduction in the frontmatter")
	}
	parts := map[string]string{}
	for _, line := range strings.Split(block, "\n") {
		if !strings.HasPrefix(line, " ") {
			break // the next top-level key
		}
		key, quoted, ok := strings.Cut(strings.TrimSpace(line), ": ")
		if !ok {
			continue
		}
		value, err := strconv.Unquote(quoted)
		if err != nil {
			return intro.Introduction{}, fmt.Errorf("introduction %s: want a quoted string: %w", key, err)
		}
		parts[key] = value
	}
	out := intro.Introduction{Purpose: parts["purpose"], Try: splitLines(parts["try"])}
	for _, l := range splitLines(parts["how"]) {
		symbol, text, _ := strings.Cut(l, " ")
		out.How = append(out.How, intro.Line{Symbol: symbol, Text: strings.TrimSpace(text)})
	}
	if err := intro.Validate(out); err != nil {
		return intro.Introduction{}, err
	}
	return out, nil
}

// splitLines is s split at line breaks; none for an empty s.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
