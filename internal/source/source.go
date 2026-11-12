// Package source owns zumen's intake store: one <id>.md file per base source
// under the settings `sourceDir` (default `00_intake`), with a controlled
// YAML-subset frontmatter this package both writes and reads: scalar `id`,
// `title`, `source` (issue|repo), `repo` (path/url), and `rev`. No YAML parser,
// no third-party dependencies.
package source

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const DEFAULT_SOURCE_DIR = "00_intake"
const SOURCE_ISSUE = "issue"
const SOURCE_REPO = "repo"

// Source is one base source: a prose issue/idea, or a pointer to an existing
// codebase.
type Source struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Source string `json:"source"` // issue | repo
	Repo   string `json:"repo"`   // path/url when source == repo
	Rev    string `json:"rev"`    // optional pin
	Body   string `json:"body"`
	Path   string `json:"path"`
}

// SourceDir resolves the store directory against the project root.
func SourceDir(root, sourceDir string) string {
	if sourceDir == "" {
		sourceDir = DEFAULT_SOURCE_DIR
	}
	return filepath.Join(root, sourceDir)
}

func SourcePath(dir string, id int) string {
	return filepath.Join(dir, fmt.Sprintf("%d.md", id))
}

var crlfRe = regexp.MustCompile(`\r\n`)

// ParseFrontmatter splits the controlled frontmatter off the raw file text,
// returning keyed-lowercase fm plus the body.
func ParseFrontmatter(raw string) (fm map[string]string, body string) {
	raw = crlfRe.ReplaceAllString(raw, "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return map[string]string{}, raw
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end < 2 {
		return map[string]string{}, raw
	}
	fm = map[string]string{}
	for _, line := range lines[1:end] {
		i := strings.Index(line, ":")
		if i == -1 {
			continue
		}
		fm[strings.ToLower(strings.TrimSpace(line[:i]))] = strings.TrimSpace(line[i+1:])
	}
	body = strings.Join(lines[end+1:], "\n")
	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimSuffix(body, "\n")
	return fm, body
}

// ParseSource parses frontmatter into a plain source object.
func ParseSource(raw string, fallbackID int) Source {
	fm, body := ParseFrontmatter(raw)
	id, _ := strconv.Atoi(fm["id"])
	if id == 0 {
		id = fallbackID
	}
	src := SOURCE_ISSUE
	if fm["source"] == SOURCE_REPO {
		src = SOURCE_REPO
	}
	return Source{
		ID:     id,
		Title:  fm["title"],
		Source: src,
		Repo:   fm["repo"],
		Rev:    fm["rev"],
		Body:   body,
	}
}

var titleWsRe = regexp.MustCompile(`\s*\r?\n+\s*`)

// ToFrontmatter serializes a source to the controlled frontmatter + body text.
func ToFrontmatter(s Source) string {
	title := strings.TrimSpace(titleWsRe.ReplaceAllString(s.Title, " "))
	out := fmt.Sprintf("---\nid: %d\ntitle: %s\nsource: %s", s.ID, title, s.Source)
	if s.Repo != "" {
		out += fmt.Sprintf("\nrepo: %s", s.Repo)
	}
	if s.Rev != "" {
		out += fmt.Sprintf("\nrev: %s", s.Rev)
	}
	out += fmt.Sprintf("\n---\n\n%s", s.Body)
	return out
}

var sourceFileRe = regexp.MustCompile(`^(\d+)\.md$`)

// maxFileId is the highest <n> among the <n>.md files in the store (0 when
// there are none).
func maxFileId(root, sourceDir string) int {
	entries, err := os.ReadDir(SourceDir(root, sourceDir))
	if err != nil {
		return 0
	}
	max := 0
	for _, e := range entries {
		if m := sourceFileRe.FindStringSubmatch(e.Name()); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > max {
				max = n
			}
		}
	}
	return max
}

// NextId is the highest numeric id present in the store + 1 (1 when empty).
// It wraps around when that id is the largest int; WriteNewSource checks for
// that, so prefer it for creating sources.
func NextId(root, sourceDir string) int { return maxFileId(root, sourceDir) + 1 }

// MaxRecordedId is the highest <n> among recorded paths of the form
// <sourceDir>/<n>.md, such as a change's sources in STATE.json. Other forms
// (issue:N, URLs, other folders) are ignored. It's 0 when there are none.
func MaxRecordedId(sourceDir string, recorded []string) int {
	dir := path.Clean(filepath.ToSlash(SourceDir("", sourceDir)))
	max := 0
	for _, r := range recorded {
		p := path.Clean(filepath.ToSlash(r))
		if path.Dir(p) != dir {
			continue
		}
		if m := sourceFileRe.FindStringSubmatch(path.Base(p)); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > max {
				max = n
			}
		}
	}
	return max
}

// ListSources returns all sources in the store, sorted by id, skipping
// non-<id>.md files.
func ListSources(root, sourceDir string) []Source {
	dir := SourceDir(root, sourceDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []Source{}
	}
	var out []Source
	for _, e := range entries {
		m := sourceFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		id, _ := strconv.Atoi(m[1])
		p := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := ParseSource(string(raw), id)
		s.Path = p
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ReadSource reads one source, erroring if the store has no such source.
func ReadSource(root, sourceDir string, id int) (Source, error) {
	p := SourcePath(SourceDir(root, sourceDir), id)
	raw, err := os.ReadFile(p)
	if err != nil {
		return Source{}, fmt.Errorf("zumen: source %d not found at %s", id, p)
	}
	s := ParseSource(string(raw), id)
	s.Path = p
	return s, nil
}

// WriteSource writes one source file. It creates the file exclusively
// (O_EXCL), so an existing file is never opened for writing, even when two runs
// pick the same id at once. An id below 1 is refused.
func WriteSource(root, sourceDir string, s Source) (Source, error) {
	if s.ID < 1 {
		return Source{}, fmt.Errorf("zumen: intake id %d out of range", s.ID)
	}
	dir := SourceDir(root, sourceDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Source{}, err
	}
	p := SourcePath(dir, s.ID)
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return Source{}, fmt.Errorf("zumen: source %d already exists at %s; not overwriting", s.ID, p)
	}
	if err != nil {
		return Source{}, err
	}
	_, werr := f.WriteString(ToFrontmatter(s) + "\n")
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(p) // ours: O_EXCL just created it
		return Source{}, werr
	}
	raw, _ := os.ReadFile(p)
	written := ParseSource(string(raw), s.ID)
	written.Path = p
	return written, nil
}

// NewSourceOptions is the payload for WriteNewSource.
type NewSourceOptions struct {
	Title  string
	Source string
	Repo   string
	Rev    string
	Body   string
	// After is an id the new source must be above: the highest intake id a
	// change still records, so a deleted intake's id is never reused. Callers
	// that create intakes pass it (the CLI's intakeFloor); 0 means files only.
	After int
}

// WriteNewSource creates a new source: next sequential id, source type derived
// from opts (repo when --repo is set, else issue). Never overwrites.
func WriteNewSource(root, sourceDir string, opts NewSourceOptions) (Source, error) {
	hi := max(maxFileId(root, sourceDir), opts.After)
	if hi >= math.MaxInt {
		return Source{}, fmt.Errorf("zumen: intake id out of range: the highest intake id (%d) leaves no next id", hi)
	}
	id := hi + 1
	src := opts.Source
	if src != SOURCE_REPO {
		src = SOURCE_ISSUE
	}
	return WriteSource(root, sourceDir, Source{
		ID:     id,
		Title:  opts.Title,
		Source: src,
		Repo:   opts.Repo,
		Rev:    opts.Rev,
		Body:   opts.Body,
	})
}
