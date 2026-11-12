// Package reference reads a workspace's references index and checks it against
// the references/ folder. The index is a user-owned markdown file, so it's
// parsed leniently: anything that isn't an entry is ignored.
package reference

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Dir is the references room, relative to the workspace root.
const Dir = "references"

// RequiredFields are the fields every index entry must have.
var RequiredFields = []string{"purpose", "from", "added"}

// Entry is one reference in the index.
type Entry struct {
	Path   string            // relative to references/, cleaned, slash-separated
	Title  string            // optional
	Fields map[string]string // purpose, from, added, summary
	Line   int               // 1-based line of the entry in the index
}

var (
	entryRe = regexp.MustCompile("^- `([^`]+)`(.*)$")
	fieldRe = regexp.MustCompile(`^\s+- (purpose|from|added|summary):[ \t]*(.*)$`)
)

// ParseIndex reads the index's entries: a top-level list item whose first
// element is a backticked path, then an optional "— title", followed by indented
// `- purpose:`, `- from:`, `- added:` and `- summary:` items. Other lines are
// ignored, so the user can add headings and prose between entries.
func ParseIndex(text string) []Entry {
	var entries []Entry
	cur := -1
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := entryRe.FindStringSubmatch(line); m != nil {
			title := strings.TrimSpace(m[2])
			title = strings.TrimSpace(strings.TrimLeft(title, "—–-"))
			entries = append(entries, Entry{
				Path:   path.Clean(filepath.ToSlash(strings.TrimSpace(m[1]))),
				Title:  title,
				Fields: map[string]string{},
				Line:   i + 1,
			})
			cur = len(entries) - 1
			continue
		}
		if m := fieldRe.FindStringSubmatch(line); m != nil && cur >= 0 {
			entries[cur].Fields[m[1]] = strings.TrimSpace(m[2])
			continue
		}
		// A non-indented, non-blank line (a heading, a paragraph) ends the entry.
		if cur >= 0 && strings.TrimSpace(line) != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			cur = -1
		}
	}
	return entries
}

// Finding is one warning from Check.
type Finding struct{ Message string }

// Result is what Check found. Present is false when there's no references/ folder.
type Result struct {
	Present  bool
	Files    int // files in the room, not counting index.md, context.md and dotfiles
	Indexed  int // how many of them the index lists
	Findings []Finding
}

// Check compares references/ under root with references/index.md. It warns about
// each file the index doesn't list, each entry whose file is missing, each
// entry missing a required field (an empty value counts as missing), and each
// entry whose path is absolute or leaves references/, which isn't looked up.
// With no index.md, every file is unindexed. With no references/ folder it finds
// nothing and Present is false.
func Check(root string) (Result, error) {
	dir := filepath.Join(root, Dir)
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		return Result{}, nil
	}
	if err != nil {
		return Result{}, err
	}
	res := Result{Present: true}

	files := map[string]bool{}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		for _, part := range strings.Split(rel, "/") {
			if strings.HasPrefix(part, ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}
		if d.IsDir() || rel == "index.md" || rel == "context.md" {
			return nil
		}
		files[rel] = true
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	indexText, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Result{}, err
	}
	listed := map[string]bool{}
	for _, e := range ParseIndex(string(indexText)) {
		if path.IsAbs(e.Path) || e.Path == ".." || strings.HasPrefix(e.Path, "../") {
			res.Findings = append(res.Findings, Finding{fmt.Sprintf("%s: outside %s/", e.Path, Dir)})
			continue
		}
		listed[e.Path] = true
		for _, field := range RequiredFields {
			if e.Fields[field] == "" {
				res.Findings = append(res.Findings, Finding{fmt.Sprintf("%s: missing %s", e.Path, field)})
			}
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(e.Path))); err != nil {
			res.Findings = append(res.Findings, Finding{fmt.Sprintf("%s: listed in the index, but the file is missing", e.Path)})
		}
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	res.Files = len(names)
	for _, name := range names {
		if listed[name] {
			res.Indexed++
			continue
		}
		res.Findings = append(res.Findings, Finding{fmt.Sprintf("%s: not in %s/index.md", name, Dir)})
	}
	return res, nil
}
