// Command docscheck keeps the Go code and docs/ honest with each other: every
// citation in a Go file must resolve to a real heading, and neither the docs
// nor the code may carry a story or epic identifier, because those name work
// items that will not outlive the tracker.
package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	// citationRe finds a pointer from code into the docs. The anchor is
	// optional: a bare file citation only has to name a file that exists.
	citationRe = regexp.MustCompile(`//\s*See\s+(docs/[^\s#]+\.md)(?:#([A-Za-z0-9_-]+))?`)
	// workItemRe matches a story or epic identifier.
	workItemRe = regexp.MustCompile(`\bS-\d{4}\b|\bE-\d{3}\b`)
	headingRe  = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*\s*$`)
	slugDrop   = regexp.MustCompile(`[^\p{L}\p{N} -]`)
)

type citation struct {
	file, line, target, anchor string
}

type heading struct {
	slug string
	text string
	line int
}

// slugify applies GitHub's anchor rule: lowercase, drop everything but
// letters, digits, spaces and hyphens, then turn spaces into hyphens.
func slugify(text string) string {
	s := slugDrop.ReplaceAllString(strings.ToLower(text), "")
	return strings.ReplaceAll(s, " ", "-")
}

// headings returns the headings of a markdown file, skipping fenced code,
// with duplicate slugs suffixed -1, -2 the way GitHub does.
func headings(path string) ([]heading, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []heading
	seen := map[string]int{}
	inFence := false
	n := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		n++
		line := sc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		m := headingRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		slug := slugify(m[1])
		if c := seen[slug]; c > 0 {
			seen[slug]++
			slug = fmt.Sprintf("%s-%d", slug, c)
		} else {
			seen[slug] = 1
		}
		out = append(out, heading{slug: slug, text: m[1], line: n})
	}
	return out, sc.Err()
}

// walk calls fn for every regular file under root, skipping dotted
// directories (.git, .cache) and any directory in skip.
func walk(root string, skip map[string]bool, fn func(path string) error) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != root && (strings.HasPrefix(name, ".") || skip[filepath.ToSlash(p)]) {
				return filepath.SkipDir
			}
			return nil
		}
		return fn(p)
	})
}

// scanLines calls fn with each line of the file and its 1-based number.
func scanLines(path string, fn func(n int, line string)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	n := 0
	for sc.Scan() {
		n++
		fn(n, sc.Text())
	}
	return sc.Err()
}

func run(root string) (failures []string, uncited []string, err error) {
	var cites []citation
	var goFiles, docFiles []string
	if err := walk(root, nil, func(p string) error {
		if strings.HasSuffix(p, ".go") {
			goFiles = append(goFiles, p)
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	docsDir := filepath.Join(root, "docs")
	if err := walk(docsDir, map[string]bool{filepath.ToSlash(filepath.Join(docsDir, "research")): true}, func(p string) error {
		if strings.HasSuffix(p, ".md") {
			docFiles = append(docFiles, p)
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}

	for _, p := range goFiles {
		rel, _ := filepath.Rel(root, p)
		err := scanLines(p, func(n int, line string) {
			pos := fmt.Sprintf("%s:%d", rel, n)
			if m := citationRe.FindStringSubmatch(line); m != nil {
				cites = append(cites, citation{file: rel, line: pos, target: m[1], anchor: m[2]})
			}
			if id := workItemRe.FindString(line); id != "" {
				failures = append(failures, fmt.Sprintf("%s: work-item identifier %s in code", pos, id))
			}
		})
		if err != nil {
			return nil, nil, err
		}
	}
	for _, p := range docFiles {
		rel, _ := filepath.Rel(root, p)
		err := scanLines(p, func(n int, line string) {
			if id := workItemRe.FindString(line); id != "" {
				failures = append(failures, fmt.Sprintf("%s:%d: work-item identifier %s in docs", rel, n, id))
			}
		})
		if err != nil {
			return nil, nil, err
		}
	}

	cache := map[string][]heading{}
	cited := map[string]bool{}
	for _, c := range cites {
		hs, ok := cache[c.target]
		if !ok {
			hs, err = headings(filepath.Join(root, filepath.FromSlash(c.target)))
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: cited file %s does not exist", c.line, c.target))
				cache[c.target] = nil
				continue
			}
			cache[c.target] = hs
		}
		if c.anchor == "" {
			continue
		}
		found := false
		for _, h := range hs {
			if h.slug == c.anchor {
				found = true
				cited[c.target+"#"+h.slug] = true
			}
		}
		if !found {
			failures = append(failures, fmt.Sprintf("%s: %s has no heading that slugifies to %q", c.line, c.target, c.anchor))
		}
	}

	for _, p := range docFiles {
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if filepath.Dir(rel) != "docs" || rel == "docs/README.md" {
			continue
		}
		hs, err := headings(p)
		if err != nil {
			return nil, nil, err
		}
		for _, h := range hs {
			if !cited[rel+"#"+h.slug] {
				uncited = append(uncited, fmt.Sprintf("%s:%d %s (#%s)", rel, h.line, h.text, h.slug))
			}
		}
	}
	sort.Strings(failures)
	return failures, uncited, nil
}

func main() {
	failures, uncited, err := run(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "docscheck:", err)
		os.Exit(1)
	}
	fmt.Printf("docscheck: %d headings in docs/*.md not cited from Go (report only)\n", len(uncited))
	for _, u := range uncited {
		fmt.Println("  uncited:", u)
	}
	if len(failures) > 0 {
		for _, f := range failures {
			fmt.Fprintln(os.Stderr, "FAIL", f)
		}
		os.Exit(1)
	}
}
