// Package adr owns zumen's architecture decision records: one NNNN-<slug>.md
// file per decision under the settings `adrDir` (default `04_architecture/adr`).
// Decisions are numbered and immutable — edit by adding a new ADR that
// supersedes the old one.
package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DEFAULT_ADR_DIR = "04_architecture/adr"
const STATUS_PROPOSED = "proposed"
const STATUS_ACCEPTED = "accepted"
const STATUS_SUPERSEDED = "superseded"

// Record is one architecture decision record.
type Record struct {
	Number   int    `json:"number"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Date     string `json:"date"`
	Deciders string `json:"deciders"`
	Path     string `json:"path"`
	Raw      string `json:"raw"`
}

// Options is the payload for Create.
type Options struct {
	Title        string
	Deciders     string
	Context      string
	Decision     string
	Consequences string
	Body         string // full body after the header; overrides the three sections
}

func AdrDir(root, adrDir string) string {
	if adrDir == "" {
		adrDir = DEFAULT_ADR_DIR
	}
	return filepath.Join(root, adrDir)
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(text string) string {
	s := strings.ToLower(text)
	s = slugNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	r := []rune(s)
	if len(r) > 40 {
		// A cut just after a "-" would leave a name adrFileRe rejects.
		s = strings.TrimRight(string(r[:40]), "-")
	}
	return s
}

var adrFileRe = regexp.MustCompile(`^(\d{4})-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$`)

// adrNumberRe matches any numbered file, valid slug or not.
var adrNumberRe = regexp.MustCompile(`^(\d{4})-.*\.md$`)

// NextNumber is the highest NNNN prefix in the store + 1 (1 when empty). It
// counts any NNNN-*.md, so a malformed name never has its number reused.
func NextNumber(root, adrDir string) int {
	dir := AdrDir(root, adrDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	max := 0
	for _, e := range entries {
		if m := adrNumberRe.FindStringSubmatch(e.Name()); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > max {
				max = n
			}
		}
	}
	return max + 1
}

func render(n int, opts Options, date string) string {
	header := fmt.Sprintf("# %04d — %s\n\n- Status: %s\n- Date: %s\n- Deciders: %s\n\n",
		n, opts.Title, STATUS_PROPOSED, date, opts.Deciders)
	if opts.Body != "" {
		return header + strings.TrimSpace(opts.Body) + "\n"
	}
	return header + fmt.Sprintf("## Context\n\n%s\n\n## Decision\n\n%s\n\n## Consequences\n\n%s\n",
		opts.Context, opts.Decision, opts.Consequences)
}

// Create writes a new ADR: next sequential number, status proposed. Never
// overwrites.
func Create(root, adrDir string, opts Options) (Record, error) {
	dir := AdrDir(root, adrDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Record{}, err
	}
	n := NextNumber(root, adrDir)
	slug := slugify(opts.Title)
	if slug == "" {
		slug = "decision"
	}
	name := fmt.Sprintf("%04d-%s.md", n, slug)
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err == nil {
		return Record{}, fmt.Errorf("zumen: adr %s already exists; not overwriting", name)
	}
	date := time.Now().Format("2006-01-02")
	if err := os.WriteFile(p, []byte(render(n, opts, date)), 0o644); err != nil {
		return Record{}, err
	}
	return Record{Number: n, Slug: slug, Title: opts.Title, Status: STATUS_PROPOSED, Date: date, Deciders: opts.Deciders, Path: p}, nil
}

// parse extracts number, title, status, date, and deciders from an ADR file's
// text.
func parse(raw string, number int, slug string) Record {
	rec := Record{Number: number, Slug: slug, Status: STATUS_PROPOSED}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "# "):
			title := strings.TrimPrefix(trimmed, "# ")
			if i := strings.Index(title, " "); i >= 0 {
				title = strings.TrimLeft(strings.TrimSpace(title[i+1:]), "—-\t ")
			}
			rec.Title = title
		case strings.HasPrefix(trimmed, "- Status:"):
			rec.Status = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Status:"))
		case strings.HasPrefix(trimmed, "- Date:"):
			rec.Date = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Date:"))
		case strings.HasPrefix(trimmed, "- Deciders:"):
			rec.Deciders = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Deciders:"))
		}
	}
	rec.Raw = raw
	return rec
}

// List returns all ADRs, sorted by number, skipping non-NNNN-slug.md files.
func List(root, adrDir string) []Record {
	dir := AdrDir(root, adrDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []Record{}
	}
	var out []Record
	for _, e := range entries {
		m := adrFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		p := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		rec := parse(string(raw), n, m[2])
		rec.Path = p
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

// Show reads one ADR by number, erroring if no such ADR exists.
func Show(root, adrDir string, number int) (Record, error) {
	for _, rec := range List(root, adrDir) {
		if rec.Number == number {
			return rec, nil
		}
	}
	return Record{}, fmt.Errorf("zumen: adr %04d not found", number)
}
