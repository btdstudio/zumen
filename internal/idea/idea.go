// Package idea owns zumen's ideas room: one ideas/<n>-<slug>/ folder per idea,
// with a summary.md whose frontmatter holds the idea's status and an append-only
// log.md. They're plain files that a person can read and edit without zumen. The
// store rewrites only the frontmatter lines it owns, and never an idea's body, and
// it never removes a folder: ideas are dropped, not deleted.
package idea

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	kiso "github.com/btdstudio/kiso/state"
	"github.com/btdstudio/zumen"
	"github.com/btdstudio/zumen/internal/source"
)

// Dir is the ideas room, relative to the workspace root.
const Dir = "ideas"

// The four statuses an idea can have.
const (
	Open     = "open"
	Parked   = "parked"
	Promoted = "promoted"
	Dropped  = "dropped"
)

// templatePath is the built-in summary template, as the ticket template is.
const templatePath = "scaffold/ideas/templates/summary.md"

// folderRe matches an idea's folder. The room also holds context.md, personas/
// and templates/, which aren't ideas.
var folderRe = regexp.MustCompile(`^(\d+)-`)

// Idea is one idea, read from its summary.md.
type Idea struct {
	ID      int
	Slug    string
	Title   string
	Status  string
	Reason  string
	Updated string   // YYYY-MM-DD
	Changes []string // the changes promoted from it, in order
	Dir     string   // ideas/<n>-<slug>, slash-separated, relative to the root
	Body    string   // summary.md below its frontmatter
}

func roomDir(root string) string { return filepath.Join(root, Dir) }

type folder struct {
	id   int
	name string
}

// folders lists the idea folders in the room, by id. A missing room is empty.
func folders(root string) ([]folder, error) {
	entries, err := os.ReadDir(roomDir(root))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []folder
	for _, e := range entries {
		m := folderRe.FindStringSubmatch(e.Name())
		if !e.IsDir() || m == nil {
			continue
		}
		if id, err := strconv.Atoi(m[1]); err == nil {
			out = append(out, folder{id, e.Name()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out, nil
}

// oneLine makes a value fit on one frontmatter line: runs of whitespace,
// newlines included, become one space, and the ends are trimmed. Otherwise a
// value could add a line of its own, such as a forged status.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func summaryPath(root string, f folder) string {
	return filepath.Join(roomDir(root), f.name, "summary.md")
}

// New creates ideas/<n>-<slug>/ with a summary from the template and a log, and
// returns the idea. The id is the highest folder number plus one, so it's never
// reused while a folder exists. The folder is made exclusively, so two runs can't
// share an id.
func New(root, title, today string) (Idea, error) {
	title = oneLine(title)
	if title == "" {
		return Idea{}, errors.New("zumen: an idea needs a title")
	}
	existing, err := folders(root)
	if err != nil {
		return Idea{}, err
	}
	highest := 0
	if len(existing) > 0 {
		highest = existing[len(existing)-1].id
	}
	if highest >= math.MaxInt {
		return Idea{}, errors.New("zumen: idea id out of range: the highest idea id leaves no next id")
	}
	id := highest + 1
	slug := kiso.Slugify(title)
	if slug == "" {
		slug = "idea"
	}
	name := fmt.Sprintf("%d-%s", id, slug)
	tmpl, err := fs.ReadFile(zumen.ScaffoldFS, templatePath)
	if err != nil {
		return Idea{}, err
	}
	summary := strings.NewReplacer("<id>", strconv.Itoa(id), "<title>", title, "<date>", today).Replace(string(tmpl))

	if err := os.MkdirAll(roomDir(root), 0o755); err != nil {
		return Idea{}, err
	}
	dir := filepath.Join(roomDir(root), name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return Idea{}, fmt.Errorf("zumen: %s/%s already exists", Dir, name)
		}
		return Idea{}, err
	}
	for rel, content := range map[string]string{
		"summary.md": summary,
		"log.md":     "# Log\n\n- " + today + " created\n",
	} {
		if err := createFile(filepath.Join(dir, rel), content); err != nil {
			os.RemoveAll(dir) // ours: the Mkdir above just made it
			return Idea{}, err
		}
	}
	return Load(root, id)
}

// createFile writes a new file and refuses to touch an existing one.
func createFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.WriteString(content)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return werr
}

// find returns the folder for id.
func find(root string, id int) (folder, error) {
	all, err := folders(root)
	if err != nil {
		return folder{}, err
	}
	var matches []folder
	for _, f := range all {
		if f.id == id {
			matches = append(matches, f)
		}
	}
	switch len(matches) {
	case 0:
		return folder{}, fmt.Errorf("zumen: idea %d not found", id)
	case 1:
		return matches[0], nil
	}
	return folder{}, fmt.Errorf("zumen: idea %d is ambiguous: more than one folder in %s/ starts with %d-", id, Dir, id)
}

// Load reads idea id from its summary.md.
func Load(root string, id int) (Idea, error) {
	f, err := find(root, id)
	if err != nil {
		return Idea{}, err
	}
	return load(root, f)
}

func load(root string, f folder) (Idea, error) {
	raw, err := os.ReadFile(summaryPath(root, f))
	if err != nil {
		return Idea{}, fmt.Errorf("zumen: cannot read %s/%s/summary.md: %v", Dir, f.name, err)
	}
	fm, body := source.ParseFrontmatter(string(raw))
	if len(fm) == 0 {
		return Idea{}, fmt.Errorf("zumen: %s/%s/summary.md has no frontmatter", Dir, f.name)
	}
	var changes []string
	for _, c := range strings.Split(fm["changes"], ",") {
		if c = strings.TrimSpace(c); c != "" {
			changes = append(changes, c)
		}
	}
	return Idea{
		ID:      f.id,
		Slug:    folderRe.ReplaceAllString(f.name, ""),
		Title:   fm["title"],
		Status:  fm["status"],
		Reason:  fm["reason"],
		Updated: fm["updated"],
		Changes: changes,
		Dir:     Dir + "/" + f.name,
		Body:    body,
	}, nil
}

// List returns every idea, by id. An idea whose summary can't be read is an
// error that names it, so it's fixed and not skipped.
func List(root string) ([]Idea, error) {
	all, err := folders(root)
	if err != nil {
		return nil, err
	}
	var out []Idea
	for _, f := range all {
		i, err := load(root, f)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, nil
}

var knownStatuses = []string{Open, Parked, Promoted, Dropped}

// change is one status change: the statuses it needs, what it sets, and what the
// log says.
type change struct {
	from   []string
	refuse func(i Idea) error // the error for an idea whose status isn't in from
	status string
	reason string // "" clears it
	extra  []kv
	log    string
}

// apply rewrites the summary's frontmatter, then appends to the log. A refusal
// changes and logs nothing.
func apply(root string, id int, today string, c change) (Idea, error) {
	f, err := find(root, id)
	if err != nil {
		return Idea{}, err
	}
	i, err := load(root, f)
	if err != nil {
		return Idea{}, err
	}
	if !slices.Contains(knownStatuses, i.Status) {
		return Idea{}, fmt.Errorf("zumen: idea %d has an unknown status %q", id, i.Status)
	}
	if !slices.Contains(c.from, i.Status) {
		return Idea{}, c.refuse(i)
	}
	path := summaryPath(root, f)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Idea{}, err
	}
	updates := append([]kv{{"status", c.status}, {"reason", c.reason}, {"updated", today}}, c.extra...)
	updated, err := setFrontmatter(string(raw), updates)
	if err != nil {
		return Idea{}, fmt.Errorf("zumen: %s/%s/summary.md: %v", Dir, f.name, err)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return Idea{}, err
	}
	if err := appendLog(root, f, today, c.log); err != nil {
		return Idea{}, fmt.Errorf("zumen: idea %d is now %s, but its log couldn't be updated: %v", id, c.status, err)
	}
	return load(root, f)
}

func requireReason(reason string) (string, error) {
	if reason = oneLine(reason); reason == "" {
		return "", errors.New("zumen: a reason is required")
	}
	return reason, nil
}

func onlyOpen(verb string) func(Idea) error {
	return func(i Idea) error {
		return fmt.Errorf("zumen: idea %d is %s: only an open idea can be %s", i.ID, i.Status, verb)
	}
}

// Park sets an open idea aside, with a reason.
func Park(root string, id int, reason, today string) (Idea, error) {
	reason, err := requireReason(reason)
	if err != nil {
		return Idea{}, err
	}
	return apply(root, id, today, change{from: []string{Open}, refuse: onlyOpen("parked"),
		status: Parked, reason: reason, log: "parked: " + reason})
}

// Drop drops an open idea, with a reason. The folder stays.
func Drop(root string, id int, reason, today string) (Idea, error) {
	reason, err := requireReason(reason)
	if err != nil {
		return Idea{}, err
	}
	return apply(root, id, today, change{from: []string{Open}, refuse: onlyOpen("dropped"),
		status: Dropped, reason: reason, log: "dropped: " + reason})
}

// Reopen returns a parked or dropped idea to open, with a reason. The reason it
// was parked or dropped for is cleared from the summary, and stays in the log.
func Reopen(root string, id int, reason, today string) (Idea, error) {
	reason, err := requireReason(reason)
	if err != nil {
		return Idea{}, err
	}
	return apply(root, id, today, change{from: []string{Parked, Dropped},
		refuse: func(i Idea) error {
			return fmt.Errorf("zumen: idea %d is %s: only a parked or dropped idea can be reopened", i.ID, i.Status)
		},
		status: Open, reason: "", log: "reopened: " + reason})
}

// MarkPromoted records that a change was promoted from the idea. It needs an open
// or already promoted idea, and adds the change to the idea's changes.
func MarkPromoted(root string, id int, changeName, today string) (Idea, error) {
	changeName = oneLine(changeName)
	if changeName == "" {
		return Idea{}, errors.New("zumen: a change name is required")
	}
	i, err := Load(root, id)
	if err != nil {
		return Idea{}, err
	}
	changes := append(slices.Clone(i.Changes), changeName)
	return apply(root, id, today, change{from: []string{Open, Promoted},
		refuse: func(i Idea) error {
			return fmt.Errorf("zumen: idea %d is %s: reopen it first", i.ID, i.Status)
		},
		status: Promoted, reason: "", extra: []kv{{"changes", strings.Join(changes, ", ")}},
		log: "promoted to " + changeName})
}

// appendLog adds one dated line to the idea's log, which is opened only for append.
func appendLog(root string, f folder, today, text string) error {
	path := filepath.Join(roomDir(root), f.name, "log.md")
	prefix := ""
	if existing, err := os.ReadFile(path); err == nil && len(existing) > 0 && existing[len(existing)-1] != '\n' {
		prefix = "\n" // a hand edit left no final newline
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	_, werr := file.WriteString(prefix + "- " + today + " " + text + "\n")
	if cerr := file.Close(); werr == nil {
		werr = cerr
	}
	return werr
}

// kv is one frontmatter key and its one-line value.
type kv struct{ key, value string }

var errNoFrontmatter = errors.New("it has no frontmatter")

// setFrontmatter replaces the value on the first line whose key matches each
// update, and adds a missing key before the closing `---`. Every other line,
// including keys a person added by hand, and everything after the block, is kept
// byte for byte. A text with no frontmatter block is an error.
func setFrontmatter(raw string, updates []kv) (string, error) {
	lines := strings.Split(raw, "\n")
	if strings.TrimRight(lines[0], "\r") != "---" {
		return "", errNoFrontmatter
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return "", errNoFrontmatter
	}
	for _, u := range updates {
		replaced := false
		for i := 1; i < end; i++ {
			j := strings.Index(lines[i], ":")
			if j < 0 || strings.ToLower(strings.TrimSpace(lines[i][:j])) != u.key {
				continue
			}
			eol := ""
			if strings.HasSuffix(lines[i], "\r") {
				eol = "\r"
			}
			lines[i] = lines[i][:j+1] + valueText(u.value) + eol
			replaced = true
			break
		}
		if !replaced {
			lines = slices.Insert(lines, end, u.key+":"+valueText(u.value))
			end++
		}
	}
	return strings.Join(lines, "\n"), nil
}

// valueText is " <value>", or nothing for an empty value, so a cleared key is
// written as `key:`.
func valueText(v string) string {
	if v == "" {
		return ""
	}
	return " " + v
}
