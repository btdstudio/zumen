package plan

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// tk is a ticket file of any type. extra is more frontmatter lines.
func tk(id, typ, change, status, extra string) string {
	repo := ""
	if typ == "story" || typ == "task" {
		repo = "repo: app\n"
	}
	return "---\nschema: 1\nid: " + id + "\ntype: " + typ + "\ntitle: " + typ + " " + id + "\n" + repo +
		"change: " + change + "\nstatus: " + status + "\n" + extra + "---\n\n## Context\nSomething.\n"
}

// problemsRoot builds a workspace from change folder → files (spec text, and
// tickets by id). An empty spec means no functional-spec.md.
func problemsRoot(t *testing.T, changes map[string]map[string]string) string {
	t.Helper()
	root := checkRoot(t)
	for change, files := range changes {
		for name, text := range files {
			p := filepath.Join(root, "specs", change, name)
			if !strings.HasSuffix(name, ".md") || name == "functional-spec.md" {
				write(t, p, text)
				continue
			}
			write(t, filepath.Join(root, "specs", change, "tickets", name), text)
		}
	}
	return root
}

func findings(t *testing.T, root string) []string {
	t.Helper()
	m, err := LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range Problems(m) {
		mark := "✗"
		if f.Severity == Warning {
			mark = "!"
		}
		out = append(out, fmt.Sprintf("%s %s %s: %s", f.Change, mark, f.ID, f.Message))
	}
	return out
}

func wantFindings(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

const spec1 = "# Functional spec — One\n\n## Functional requirements\n\n" +
	"- R1. A. — **must**\n- R2. B. — **must**\n- R3. C. — **should**\n- R4. D.\n- N1. E. — **must**\n"

// AC1: an unknown citation is an error; with no spec, every citation is.
func TestProblemsUnknownCitation(t *testing.T) {
	root := problemsRoot(t, map[string]map[string]string{
		"1-one": {
			"functional-spec.md": spec1,
			"S-0001.md":          tk("S-0001", "story", "1-one", "draft", "requirements: [R1, R99, R2]\n"),
		},
		"2-two": {
			"S-0002.md": tk("S-0002", "story", "2-two", "draft", "requirements: [R1]\n"),
		},
	})
	wantFindings(t, findings(t, root),
		"1-one ! R4: spec of 1-one: line 8: R4 has no priority (expected — **must**, — **should** or — **could**)",
		"1-one ✗ S-0001: cites R99, which isn't in 1-one's spec",
		"2-two ✗ S-0002: cites R1, but 2-two has no spec",
	)
}

// AC2: only uncited must R requirements warn, and only in a change with tickets.
func TestProblemsCoverage(t *testing.T) {
	noPriority := strings.Replace(spec1, "- R4. D.\n", "", 1)
	root := problemsRoot(t, map[string]map[string]string{
		"1-one": {
			"functional-spec.md": noPriority,
			"F-0001.md":          tk("F-0001", "feature", "1-one", "draft", ""),
			"S-0002.md":          tk("S-0002", "story", "1-one", "draft", "parent: F-0001\nrequirements: [R2]\n"),
		},
		"2-two": {
			"functional-spec.md": noPriority, // no tickets: no coverage warnings
		},
	})
	wantFindings(t, findings(t, root),
		"1-one ! R1: must requirement in 1-one isn't cited by any ticket",
	)
}

// AC3: ready stories and tasks need requirements; epics, features and drafts don't.
func TestProblemsReadyWithoutRequirements(t *testing.T) {
	root := problemsRoot(t, map[string]map[string]string{
		"1-one": {
			"functional-spec.md": "## Functional requirements\n\n- R1. A. — **should**\n",
			"E-0001.md":          tk("E-0001", "epic", "1-one", "ready", ""),
			"F-0002.md":          tk("F-0002", "feature", "1-one", "ready", ""),
			"S-0003.md":          tk("S-0003", "story", "1-one", "ready", ""),
			"S-0004.md":          tk("S-0004", "story", "1-one", "draft", ""),
			"S-0005.md":          tk("S-0005", "story", "1-one", "ready", "requirements: [R1]\n"),
			"T-0006.md":          tk("T-0006", "task", "1-one", "ready", ""),
			"S-0007.md":          tk("S-0007", "story", "1-one", "published", ""),
		},
	})
	wantFindings(t, findings(t, root),
		"1-one ! S-0003: ready story with no requirements",
		"1-one ! T-0006: ready task with no requirements",
	)
}

// AC4: malformed and unresolved ADR citations warn; a custom adrDir is honored.
func TestProblemsADRCitations(t *testing.T) {
	root := problemsRoot(t, map[string]map[string]string{
		"1-one": {
			"S-0001.md": tk("S-0001", "story", "1-one", "draft", "adrs: [ADR-0001, ADR-0009, \"0001\", adr-0001]\n"),
		},
	})
	write(t, filepath.Join(root, ".zumen", "settings.json"), `{"adrDir": "decisions"}`)
	write(t, filepath.Join(root, "decisions", "0001-first.md"), "# 0001 — First\n")
	write(t, filepath.Join(root, "04_architecture", "adr", "0009-wrong-folder.md"), "# 0009 — Elsewhere\n")
	wantFindings(t, findings(t, root),
		"1-one ! S-0001: cites ADR-0009, but there's no 0009-*.md ADR file",
		`1-one ! S-0001: adrs entry "0001" isn't of the form ADR-<nnnn>`,
		`1-one ! S-0001: adrs entry "adr-0001" isn't of the form ADR-<nnnn>`,
	)
}

// AC6: the spec's own findings come through, naming the change.
func TestProblemsSpecFindings(t *testing.T) {
	root := problemsRoot(t, map[string]map[string]string{
		"1-one": {
			"functional-spec.md": "## Functional requirements\n\n- R2. A. — **should**\n- R2. Again. — **should**\n- R4. B.\n",
		},
	})
	wantFindings(t, findings(t, root),
		"1-one ✗ R2: spec of 1-one: line 4: R2 is already defined on line 3",
		"1-one ! R4: spec of 1-one: line 5: R4 has no priority (expected — **must**, — **should** or — **could**)",
	)
}

// AC8: the order across changes and within one, and citations resolve only
// against the ticket's own change.
func TestProblemsOrderAndScope(t *testing.T) {
	spec := "## Functional requirements\n\n- R1. A. — **must**\n- R2. B. — **must**\n"
	root := problemsRoot(t, map[string]map[string]string{
		"10-later": {
			"functional-spec.md": spec,
			"S-0010.md":          tk("S-0010", "story", "10-later", "draft", "requirements: [R1, R2]\n"),
		},
		"2-earlier": {
			"functional-spec.md": strings.Replace(spec, "- R2. B. — **must**\n", "- R2. B.\n", 1),
			"S-0009.md":          tk("S-0009", "story", "2-earlier", "ready", "adrs: [ADR-0007]\n"),
			"S-0003.md":          tk("S-0003", "story", "2-earlier", "draft", "requirements: [R5]\nadrs: [x]\n"),
		},
	})
	want := []string{
		"2-earlier ! R2: spec of 2-earlier: line 4: R2 has no priority (expected — **must**, — **should** or — **could**)",
		"2-earlier ✗ S-0003: cites R5, which isn't in 2-earlier's spec",
		"2-earlier ! S-0009: ready story with no requirements",
		"2-earlier ! R1: must requirement in 2-earlier isn't cited by any ticket",
		`2-earlier ! S-0003: adrs entry "x" isn't of the form ADR-<nnnn>`,
		"2-earlier ! S-0009: cites ADR-0007, but there's no 0007-*.md ADR file",
	}
	wantFindings(t, findings(t, root), want...)
	wantFindings(t, findings(t, root), want...) // the same on every run
}

func TestLoadAllMatchesLoad(t *testing.T) {
	root := workspace(t)
	all, err := LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	one, err := Load(root, "1-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if all.Change != "" || all.HasSpec || all.Requirements != nil {
		t.Fatalf("LoadAll set single-change fields: %+v", all)
	}
	if !reflect.DeepEqual(all.Tickets, one.Tickets) || !reflect.DeepEqual(all.ADRs, one.ADRs) || !reflect.DeepEqual(all.Changes, one.Changes) {
		t.Fatal("LoadAll and Load disagree on the workspace-wide fields")
	}
	var names []string
	for _, cs := range all.Changes {
		names = append(names, cs.Name)
	}
	if !reflect.DeepEqual(names, []string{"1-alpha", "10-beta"}) || !all.Changes[0].HasSpec || all.Changes[1].HasSpec {
		t.Fatalf("changes = %+v", all.Changes)
	}
	if all.Changes[0].Title != "Alpha things" || len(all.Changes[0].Requirements) != 3 {
		t.Fatalf("1-alpha = %+v", all.Changes[0])
	}
	if !reflect.DeepEqual(one.Requirements, all.Changes[0].Requirements) {
		t.Fatal("Load's requirements differ from its ChangeSpec's")
	}
}
