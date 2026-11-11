// Package plan is zumen's read model of a change: the requirements in its
// functional spec, the diagrams in its architecture, every ticket in the
// workspace and the ADRs. The gate checks, `tickets check` coverage and the
// overview all read it, so they agree on what's covered or broken.
//
// plan sits below internal/state: it imports internal/adr, internal/settings
// and kiso's ticket and placeholder packages, never state or cli.
package plan

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/btdstudio/kiso/ticket"
	"github.com/btdstudio/zumen/internal/adr"
	"github.com/btdstudio/zumen/internal/settings"
)

// specsDir is the folder of change folders. internal/state's store uses the
// same name; plan can't import state, so a test pins the two together.
const specsDir = "specs"

// Model is the read model of one change, plus the workspace-wide tickets and
// ADRs its checks need.
type Model struct {
	Change string
	Title  string // the spec's H1 after "— ", or the change name

	HasSpec      bool // functional-spec.md exists
	Requirements []Requirement
	SpecFindings []Finding

	HasArchitecture bool // architecture.md exists
	DiagramSection  bool // architecture.md has a ## Diagrams heading
	Diagrams        []Diagram

	Tickets        []*ticket.Ticket // the whole workspace, sorted by id
	TicketProblems []ticket.Problem // ticket files that couldn't be parsed

	ADRs map[string]adr.Record // "ADR-0001" → record

	Changes []ChangeSpec // every numbered change's spec, by number
}

// ChangeSpec is one change's parsed functional spec.
type ChangeSpec struct {
	Name         string
	HasSpec      bool
	Title        string // the H1 after "— ", or "" without an H1
	Requirements []Requirement
	Findings     []Finding // ParseSpec's; Change isn't set
}

// LoadAll reads what the whole workspace shares: every numbered change's spec,
// every ticket, and the ADRs. Change and the single-change fields stay empty.
// Given the same files, it returns the same Model.
func LoadAll(root string) (*Model, error) {
	m := &Model{ADRs: map[string]adr.Record{}}
	names, err := changeNames(root)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		cs, err := loadSpec(root, name)
		if err != nil {
			return nil, err
		}
		m.Changes = append(m.Changes, cs)
		ts, bad, err := ticket.LoadDir(filepath.Join(root, specsDir, name, "tickets"))
		if err != nil {
			return nil, err
		}
		m.Tickets = append(m.Tickets, ts...)
		m.TicketProblems = append(m.TicketProblems, bad...)
	}
	sort.SliceStable(m.Tickets, func(i, j int) bool { return m.Tickets[i].ID < m.Tickets[j].ID })

	loaded, err := settings.LoadSettings(root)
	if err != nil {
		return nil, err
	}
	for _, r := range adr.List(root, loaded.Effective.AdrDir) {
		key := fmt.Sprintf("ADR-%04d", r.Number)
		// adr.List's sort isn't stable: keep the smaller path for a shared number.
		if prev, dup := m.ADRs[key]; dup && prev.Path < r.Path {
			continue
		}
		m.ADRs[key] = r
	}
	return m, nil
}

// Load is LoadAll plus one change's spec, title and architecture. A missing
// spec or architecture isn't an error: its Has flag is false and its fields
// are empty.
func Load(root, change string) (*Model, error) {
	if change == "" {
		return nil, errors.New("zumen: plan: no change given")
	}
	m, err := LoadAll(root)
	if err != nil {
		return nil, err
	}
	m.Change, m.Title = change, change
	cs, ok := m.spec(change)
	if !ok {
		// Not a numbered change folder, so LoadAll didn't read it.
		if cs, err = loadSpec(root, change); err != nil {
			return nil, err
		}
	}
	if cs.HasSpec {
		m.HasSpec = true
		m.Requirements, m.SpecFindings = cs.Requirements, cs.Findings
		if cs.Title != "" {
			m.Title = cs.Title
		}
	}

	arch, ok, err := readIfExists(filepath.Join(root, specsDir, change, "architecture.md"))
	if err != nil {
		return nil, err
	}
	if ok {
		m.HasArchitecture = true
		m.DiagramSection, m.Diagrams = Diagrams(arch)
	}
	return m, nil
}

// spec returns a change's entry in Changes.
func (m *Model) spec(change string) (ChangeSpec, bool) {
	for _, cs := range m.Changes {
		if cs.Name == change {
			return cs, true
		}
	}
	return ChangeSpec{}, false
}

// loadSpec reads and parses a change's functional spec, if it has one.
func loadSpec(root, change string) (ChangeSpec, error) {
	cs := ChangeSpec{Name: change}
	text, ok, err := readIfExists(filepath.Join(root, specsDir, change, "functional-spec.md"))
	if err != nil || !ok {
		return cs, err
	}
	cs.HasSpec = true
	cs.Title = title(text)
	cs.Requirements, cs.Findings = ParseSpec(text)
	return cs, nil
}

// readIfExists reads a file; ok is false if it doesn't exist.
func readIfExists(path string) (text string, ok bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(data), true, nil
}

// title is the text after the first "— " of the document's first H1 outside
// fences and comments, or the whole H1 text without one.
func title(markdown string) string {
	for _, l := range scan(markdown) {
		if !l.prose() || !strings.HasPrefix(l.text, "# ") {
			continue
		}
		h := strings.TrimSpace(l.text[2:])
		if _, after, found := strings.Cut(h, "— "); found {
			return strings.TrimSpace(after)
		}
		return h
	}
	return ""
}

var changeDirRe = regexp.MustCompile(`^(\d+)-`)

// changeNames lists the numbered change folders under specs/, by number then
// name. A missing specs/ has none.
func changeNames(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, specsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	type change struct {
		id   int
		name string
	}
	var changes []change
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if mm := changeDirRe.FindStringSubmatch(e.Name()); mm != nil {
			id, _ := strconv.Atoi(mm[1])
			changes = append(changes, change{id, e.Name()})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].id != changes[j].id {
			return changes[i].id < changes[j].id
		}
		return changes[i].name < changes[j].name
	})
	names := make([]string, len(changes))
	for i, c := range changes {
		names[i] = c.name
	}
	return names, nil
}
