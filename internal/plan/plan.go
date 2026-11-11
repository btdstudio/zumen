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
}

// Load reads a change's functional spec and architecture, every ticket in the
// workspace, and the ADRs. A missing spec or architecture isn't an error: its
// Has flag is false and its fields are empty. Given the same files, Load
// returns the same Model.
func Load(root, change string) (*Model, error) {
	if change == "" {
		return nil, errors.New("zumen: plan: no change given")
	}
	m := &Model{Change: change, Title: change, ADRs: map[string]adr.Record{}}

	spec, ok, err := readIfExists(filepath.Join(root, specsDir, change, "functional-spec.md"))
	if err != nil {
		return nil, err
	}
	if ok {
		m.HasSpec = true
		m.Requirements, m.SpecFindings = ParseSpec(spec)
		if t := title(spec); t != "" {
			m.Title = t
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

	if m.Tickets, m.TicketProblems, err = loadTickets(root); err != nil {
		return nil, err
	}

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
// a fence, or the whole H1 text without one.
func title(markdown string) string {
	for _, l := range scan(markdown) {
		if l.inFence || !strings.HasPrefix(l.text, "# ") {
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

// loadTickets reads the tickets of every numbered change folder, sorted by id.
func loadTickets(root string) ([]*ticket.Ticket, []ticket.Problem, error) {
	entries, err := os.ReadDir(filepath.Join(root, specsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
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
	var all []*ticket.Ticket
	var problems []ticket.Problem
	for _, c := range changes {
		ts, bad, err := ticket.LoadDir(filepath.Join(root, specsDir, c.name, "tickets"))
		if err != nil {
			return nil, nil, err
		}
		all = append(all, ts...)
		problems = append(problems, bad...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return all, problems, nil
}
