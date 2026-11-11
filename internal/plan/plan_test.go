package plan

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ticketText(id, typ, change string) string {
	return "---\nschema: 1\nid: " + id + "\ntype: " + typ + "\ntitle: Ticket " + id +
		"\nchange: " + change + "\nstatus: draft\n---\n\n## Context\nSomething.\n"
}

// workspace builds AC8's scratch workspace: two changes with tickets in each
// (and one unparsable file), two ADRs in a custom adrDir, and a spec and
// architecture in the first change only.
func workspace(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	a := filepath.Join(root, "specs", "1-alpha")
	b := filepath.Join(root, "specs", "10-beta")
	write(t, filepath.Join(a, "functional-spec.md"),
		"# Functional spec — Alpha things\n\n## Functional requirements\n\n- R1. One. — **must**\n- R2. Two.\n- N1. Three. — **should**\n")
	write(t, filepath.Join(a, "architecture.md"),
		"# Architecture — Alpha things\n\n## Diagrams\n\n### Context\n```mermaid\nflowchart LR\n  a --> b\n```\n")
	write(t, filepath.Join(a, "tickets", "S-0003.md"), ticketText("S-0003", "story", "1-alpha"))
	write(t, filepath.Join(a, "tickets", "F-0001.md"), ticketText("F-0001", "feature", "1-alpha"))
	write(t, filepath.Join(a, "tickets", "notes.txt"), "not a ticket")
	write(t, filepath.Join(b, "tickets", "S-0002.md"), ticketText("S-0002", "story", "10-beta"))
	write(t, filepath.Join(b, "tickets", "S-0009.md"), "---\nschema: [\n---\n")
	write(t, filepath.Join(root, "specs", "notes.md"), "not a change")
	write(t, filepath.Join(root, "specs", "draft", "tickets", "S-0099.md"), ticketText("S-0099", "story", "draft"))
	write(t, filepath.Join(root, ".zumen", "settings.json"), `{"adrDir": "decisions"}`)
	write(t, filepath.Join(root, "decisions", "0001-first.md"), "# 0001 — First\n\n- Status: accepted\n")
	write(t, filepath.Join(root, "decisions", "0002-second.md"), "# 0002 — Second\n\n- Status: proposed\n")
	write(t, filepath.Join(root, "decisions", "README.md"), "not an ADR")
	return root
}

func ticketIDs(m *Model) []string {
	var ids []string
	for _, tk := range m.Tickets {
		ids = append(ids, tk.ID)
	}
	return ids
}

// AC8.
func TestLoad(t *testing.T) {
	root := workspace(t)

	m, err := Load(root, "1-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if m.Change != "1-alpha" || m.Title != "Alpha things" {
		t.Errorf("change, title = %q, %q", m.Change, m.Title)
	}
	if !m.HasSpec || !m.HasArchitecture || !m.DiagramSection {
		t.Errorf("flags = %v %v %v", m.HasSpec, m.HasArchitecture, m.DiagramSection)
	}
	wantReqs := []Requirement{{"R1", 'R', "must", 5}, {"R2", 'R', "", 6}, {"N1", 'N', "should", 7}}
	if !reflect.DeepEqual(m.Requirements, wantReqs) {
		t.Errorf("requirements = %+v", m.Requirements)
	}
	if len(m.SpecFindings) != 1 || m.SpecFindings[0].ID != "R2" || m.SpecFindings[0].Severity != Warning {
		t.Errorf("spec findings = %+v", m.SpecFindings)
	}
	wantDiagrams := []Diagram{{"Context", 5, "flowchart LR\n  a --> b"}}
	if !reflect.DeepEqual(m.Diagrams, wantDiagrams) {
		t.Errorf("diagrams = %+v", m.Diagrams)
	}
	if got := ticketIDs(m); !reflect.DeepEqual(got, []string{"F-0001", "S-0002", "S-0003"}) {
		t.Errorf("tickets = %v", got)
	}
	if len(m.TicketProblems) != 1 || !strings.HasSuffix(m.TicketProblems[0].ID, "S-0009.md") {
		t.Errorf("ticket problems = %+v", m.TicketProblems)
	}
	if len(m.ADRs) != 2 || m.ADRs["ADR-0001"].Title != "First" || m.ADRs["ADR-0002"].Status != "proposed" {
		t.Errorf("adrs = %+v", m.ADRs)
	}

	other, err := Load(root, "10-beta")
	if err != nil {
		t.Fatal(err)
	}
	if other.HasSpec || other.HasArchitecture || other.DiagramSection {
		t.Errorf("10-beta flags = %v %v %v", other.HasSpec, other.HasArchitecture, other.DiagramSection)
	}
	if other.Title != "10-beta" || other.Requirements != nil || other.Diagrams != nil {
		t.Errorf("10-beta = %+v", other)
	}
	if !reflect.DeepEqual(ticketIDs(other), ticketIDs(m)) || len(other.ADRs) != 2 {
		t.Errorf("10-beta tickets, adrs = %v, %d", ticketIDs(other), len(other.ADRs))
	}

	// R7: the same files give the same model.
	again, err := Load(root, "1-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, m) {
		t.Errorf("second load differs:\n%+v\n%+v", again, m)
	}
}

// A spec without an H1 keeps the change name as the title. Only numbered
// directories are changes: a numbered file in specs/ is skipped, not read as
// a change folder. Changes are read in numeric order, which orders problems.
func TestLoadChangeFolders(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	write(t, filepath.Join(root, "specs", "10-y", "functional-spec.md"), "## Functional requirements\n\n- R1. One. — **must**\n")
	write(t, filepath.Join(root, "specs", "10-y", "tickets", "S-0001.md"), "---\nschema: [\n---\n")
	write(t, filepath.Join(root, "specs", "2-x", "tickets", "S-0002.md"), "---\nschema: [\n---\n")
	write(t, filepath.Join(root, "specs", "3-notes.md"), "a numbered file, not a change")
	m, err := Load(root, "10-y")
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "10-y" || len(m.Requirements) != 1 {
		t.Errorf("title, requirements = %q, %+v", m.Title, m.Requirements)
	}
	var got []string
	for _, p := range m.TicketProblems {
		got = append(got, filepath.Base(filepath.Dir(filepath.Dir(p.ID))))
	}
	if !reflect.DeepEqual(got, []string{"2-x", "10-y"}) {
		t.Errorf("problem order = %v", got)
	}
}

func TestLoadEmptyWorkspace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, err := Load(t.TempDir(), "1-x")
	if err != nil {
		t.Fatal(err)
	}
	if m.HasSpec || m.HasArchitecture || m.Tickets != nil || len(m.ADRs) != 0 || m.Title != "1-x" {
		t.Fatalf("model = %+v", m)
	}
}

func TestLoadEmptyChange(t *testing.T) {
	if _, err := Load(t.TempDir(), ""); err == nil || !strings.Contains(err.Error(), "no change given") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadReadError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	// A directory where the spec should be can't be read.
	if err := os.MkdirAll(filepath.Join(root, "specs", "1-x", "functional-spec.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, "1-x"); err == nil {
		t.Fatal("want an error")
	}
}

func TestLoadBadSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	write(t, filepath.Join(root, ".zumen", "settings.json"), "{")
	if _, err := Load(root, "1-x"); err == nil {
		t.Fatal("want an error")
	}
}

func TestTitle(t *testing.T) {
	cases := map[string]string{
		"# Functional spec — Eval fixes\n":    "Eval fixes",
		"# Plain title\n":                     "Plain title",
		"# Spec —   Spaced  \n":               "Spaced",
		"<!--\n# Commented\n-->\n# Real\n":    "Real",
		"intro\n```\n# fenced\n```\n# Real\n": "Real",
		"## Not an H1\n":                      "",
		"":                                    "",
	}
	for doc, want := range cases {
		if got := title(doc); got != want {
			t.Errorf("title(%q) = %q, want %q", doc, got, want)
		}
	}
}

func TestDuplicateADRNumberKeepsSmallerPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	dir := filepath.Join(root, "04_architecture", "adr")
	write(t, filepath.Join(dir, "0003-b.md"), "# 0003 — B\n")
	write(t, filepath.Join(dir, "0003-a.md"), "# 0003 — A\n")
	m, err := Load(root, "1-x")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.ADRs["ADR-0003"].Title; got != "A" {
		t.Fatalf("ADR-0003 = %q, want A", got)
	}
}

func TestSpecsDir(t *testing.T) {
	if specsDir != "specs" {
		t.Fatalf("specsDir = %q", specsDir)
	}
}
