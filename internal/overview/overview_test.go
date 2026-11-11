package overview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/zumen/internal/plan"
)

var phases = []string{"intake", "research", "explore", "synthesis", "spec", "architecture", "breakdown", "publish"}

// tk is a ticket file. extra is more frontmatter lines.
func tk(id, typ, title, change, status, extra string) string {
	repo := ""
	if typ == "story" || typ == "task" {
		repo = "repo: app\n"
	}
	return fmt.Sprintf("---\nschema: 1\nid: %s\ntype: %s\ntitle: %q\n%schange: %s\nstatus: %s\n%s---\n\n## Context\nSomething.\n\n## Acceptance criteria\n- [ ] AC1: it works\n",
		id, typ, title, repo, change, status, extra)
}

// load writes files (paths relative to the root) and loads change 1-auth.
func load(t *testing.T, files map[string]string) *plan.Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	for p, text := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "specs", "1-auth")); err != nil {
		if err := os.MkdirAll(filepath.Join(root, "specs", "1-auth"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	m, err := plan.Load(root, "1-auth")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func render(m *plan.Model, gates map[string]string, narrativeEmpty bool) string {
	return Render(Input{Model: m, Phases: phases, Gates: gates, Reasons: map[string]string{"research": "nothing | to find"},
		Repos: []string{"app"}, ADRDir: "04_architecture/adr", NarrativeEmpty: narrativeEmpty})
}

// section returns one section's body.
func section(t *testing.T, out, title string) string {
	t.Helper()
	start := strings.Index(out, "\n## "+title+"\n\n")
	if start < 0 {
		t.Fatalf("no %s section:\n%s", title, out)
	}
	body := out[start+len("\n## "+title+"\n\n"):]
	if end := strings.Index(body, "\n\n## "); end >= 0 {
		body = body[:end]
	}
	return strings.TrimSuffix(body, "\n\n")
}

func wantSection(t *testing.T, out, title, want string) {
	t.Helper()
	if got := section(t, out, title); got != want {
		t.Fatalf("%s:\n%s\nwant:\n%s", title, got, want)
	}
}

func TestRenderEmptyChange(t *testing.T) {
	out := render(load(t, nil), nil, false)
	if !strings.HasPrefix(out, "\n## Status\n\n") || !strings.HasSuffix(out, "\n\n") {
		t.Fatalf("region framing:\n%q", out)
	}
	var titles []string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "## ") {
			titles = append(titles, l[3:])
		}
	}
	if strings.Join(titles, ",") != "Status,Diagrams,Work map,Dependencies,Repos,Decisions,Traceability,Problems" {
		t.Fatalf("sections = %v", titles)
	}
	for _, s := range titles[1:] {
		wantSection(t, out, s, none)
	}
}

func TestRenderStatus(t *testing.T) {
	out := render(load(t, nil), map[string]string{"intake": "approved", "research": "skipped"}, false)
	want := "| Phase | Status | Note |\n|---|---|---|\n" +
		"| intake | approved |  |\n| research | skipped | nothing \\| to find |\n" +
		"| explore | — |  |\n| synthesis | — |  |\n| spec | — |  |\n| architecture | — |  |\n| breakdown | — |  |\n| publish | — |  |"
	wantSection(t, out, "Status", want)
}

func TestRenderDiagrams(t *testing.T) {
	arch := "# A\n\n## Diagrams\n\n### Context\n\n```mermaid\nflowchart LR\n  a --> b\n```\n\n### Layers\n\nLater.\n"
	out := render(load(t, map[string]string{"specs/1-auth/architecture.md": arch}), nil, false)
	wantSection(t, out, "Diagrams", "### Context\n\n```mermaid\nflowchart LR\n  a --> b\n```\n\n### Layers\n\n_No diagram yet._\n\nFrom [architecture.md](architecture.md).")

	out = render(load(t, map[string]string{"specs/1-auth/architecture.md": "# A\n"}), nil, false)
	wantSection(t, out, "Diagrams", none+"\n\nFrom [architecture.md](architecture.md).")
}

func TestRenderWorkMap(t *testing.T) {
	m := load(t, map[string]string{
		"specs/1-auth/tickets/E-0001.md": tk("E-0001", "epic", "Accounts", "1-auth", "draft", ""),
		"specs/1-auth/tickets/S-0003.md": tk("S-0003", "story", "Sign out", "1-auth", "ready", "parent: E-0001\nrequirements: [R2]\n"),
		"specs/1-auth/tickets/S-0002.md": tk("S-0002", "story", "Sign in", "1-auth", "published",
			"parent: E-0001\nrequirements: [R1, N1]\nexternal:\n  jira: AUTH-7\n  github: o/app#7\npublished_hash: \"v2:x\"\n"),
		"specs/1-auth/tickets/T-0004.md":  tk("T-0004", "task", "Child of another change", "1-auth", "draft", "parent: F-0009\n"),
		"specs/1-auth/tickets/S-0005.md":  tk("S-0005", "story", "Loop A", "1-auth", "draft", "parent: S-0006\n"),
		"specs/1-auth/tickets/S-0006.md":  tk("S-0006", "story", "Loop B", "1-auth", "draft", "parent: S-0005\n"),
		"specs/2-other/tickets/F-0009.md": tk("F-0009", "feature", "Elsewhere", "2-other", "draft", ""),
	})
	want := "- [**E-0001**](tickets/E-0001.md) Accounts · draft\n" +
		"  - [**S-0002**](tickets/S-0002.md) Sign in · `app` · published · github o/app#7, jira AUTH-7 · R1, N1\n" +
		"  - [**S-0003**](tickets/S-0003.md) Sign out · `app` · ready · R2\n" +
		"- [**T-0004**](tickets/T-0004.md) Child of another change · `app` · draft\n" +
		"- [**S-0005**](tickets/S-0005.md) Loop A · `app` · draft\n" +
		"  - [**S-0006**](tickets/S-0006.md) Loop B · `app` · draft"
	wantSection(t, render(m, nil, false), "Work map", want)
}

// AC5, AC6.
func TestRenderDependencies(t *testing.T) {
	m := load(t, map[string]string{
		"specs/1-auth/tickets/E-0001.md":  tk("E-0001", "epic", "Accounts", "1-auth", "draft", ""),
		"specs/1-auth/tickets/S-0002.md":  tk("S-0002", "story", `Sign "in"`, "1-auth", "draft", "parent: E-0001\ndepends_on: [S-0008, S-0003, S-0099]\n"),
		"specs/1-auth/tickets/S-0003.md":  tk("S-0003", "story", "A title that is much longer than forty characters in all", "1-auth", "draft", "parent: E-0001\n"),
		"specs/1-auth/tickets/S-0004.md":  tk("S-0004", "story", "Loose", "1-auth", "draft", "depends_on: [S-0002]\n"),
		"specs/1-auth/tickets/S-0010.md":  tk("S-0010", "story", "Alone", "1-auth", "draft", ""),
		"specs/2-other/tickets/S-0008.md": tk("S-0008", "story", "Session store", "2-other", "draft", ""),
	})
	out := render(m, nil, false)
	want := "### E-0001 · Accounts\n\n```mermaid\nflowchart LR\n" +
		"  E_0001[\"E-0001 · Accounts\"]\n" +
		"  S_0002[\"S-0002 · Sign #quot;in#quot;\"]\n" +
		"  S_0003[\"S-0003 · A title that is much longer than forty…\"]\n" +
		"  S_0008[\"S-0008 · Session store (2-other)\"]:::external\n" +
		"  S_0099[\"S-0099 · not found\"]:::external\n" +
		"  S_0003 --> S_0002\n  S_0008 --> S_0002\n  S_0099 --> S_0002\n" +
		"  classDef external stroke-dasharray: 5 5\n```\n\n" +
		"### No epic\n\n```mermaid\nflowchart LR\n" +
		"  S_0004[\"S-0004 · Loose\"]\n  S_0010[\"S-0010 · Alone\"]\n" +
		"  S_0002[\"S-0002 · Sign #quot;in#quot;\"]:::external\n" +
		"  S_0002 --> S_0004\n" +
		"  classDef external stroke-dasharray: 5 5\n```"
	wantSection(t, out, "Dependencies", want)
	if strings.Contains(section(t, out, "Work map"), "S-0008") {
		t.Fatal("the Work map lists another change's ticket")
	}
}

// AC6: more than 25 nodes, externals included, is a table; 25 is Mermaid.
func TestRenderDependenciesTableFallback(t *testing.T) {
	for _, c := range []struct {
		stories int
		table   bool
	}{{23, false}, {24, true}} {
		files := map[string]string{
			"specs/1-auth/tickets/E-0001.md":  tk("E-0001", "epic", "Big", "1-auth", "draft", ""),
			"specs/2-other/tickets/S-0900.md": tk("S-0900", "story", "Outside", "2-other", "draft", ""),
		}
		for i := 1; i <= c.stories; i++ {
			id := fmt.Sprintf("S-%04d", i+1)
			extra := "parent: E-0001\n"
			if i == 1 {
				extra += "depends_on: [S-0900]\n"
			}
			files["specs/1-auth/tickets/"+id+".md"] = tk(id, "story", "Story "+id, "1-auth", "draft", extra)
		}
		body := section(t, render(load(t, files), nil, false), "Dependencies")
		nodes := c.stories + 2 // the epic and the external
		if isTable := strings.Contains(body, "| Ticket | Depends on |"); isTable != c.table {
			t.Fatalf("%d nodes: table = %v\n%s", nodes, isTable, body)
		}
		if c.table && !strings.Contains(body, "| [S-0002](tickets/S-0002.md) | S-0900 (2-other) |") {
			t.Fatalf("table row:\n%s", body)
		}
	}
}

func TestRenderReposDecisionsTraceability(t *testing.T) {
	m := load(t, map[string]string{
		"specs/1-auth/functional-spec.md": "# Functional spec — Auth\n\n## Functional requirements\n\n" +
			"- R1. A. — **must**\n- R2. B. — **must**\n- R3. C. — **should**\n- R4. D.\n- N1. E. — **must**\n",
		"specs/1-auth/tickets/E-0001.md":         tk("E-0001", "epic", "Accounts", "1-auth", "draft", "adrs: [ADR-0001]\n"),
		"specs/1-auth/tickets/S-0002.md":         tk("S-0002", "story", "Sign in", "1-auth", "ready", "requirements: [R1, N1]\nadrs: [ADR-0001, ADR-0009]\n"),
		"specs/1-auth/tickets/S-0003.md":         tk("S-0003", "story", "Sign out", "1-auth", "draft", "requirements: [R1]\nadrs: [\"0002\"]\n"),
		"04_architecture/adr/0001-use-tokens.md": "# 0001 — Use tokens\n\n- Status: accepted\n",
	})
	out := render(m, nil, false)
	wantSection(t, out, "Repos", "| Repo | draft | ready | published | Tickets |\n|---|---|---|---|---|\n| `app` | 1 | 1 | 0 | S-0002, S-0003 |")
	wantSection(t, out, "Decisions", "| ADR | Title | Status | Cited by |\n|---|---|---|---|\n"+
		"| 0002 | not found |  | S-0003 |\n"+
		"| [ADR-0001](../../04_architecture/adr/0001-use-tokens.md) | Use tokens | accepted | E-0001, S-0002 |\n"+
		"| ADR-0009 | not found |  | S-0002 |")
	wantSection(t, out, "Traceability", "From [functional-spec.md](functional-spec.md).\n\n| Req | Priority | Covered by |\n|---|---|---|\n"+
		"| R1 | must | [S-0002](tickets/S-0002.md), [S-0003](tickets/S-0003.md) |\n"+
		"| R2 | must | **uncovered** |\n| R3 | should |  |\n| R4 | — |  |\n"+
		"| N1 | must | [S-0002](tickets/S-0002.md) |")
}

// AC7, and validation, unparsable files, plan findings, sorting and no paths.
func TestRenderProblems(t *testing.T) {
	m := load(t, map[string]string{
		"specs/1-auth/functional-spec.md": "## Functional requirements\n\n- R1. A. — **must**\n",
		"specs/1-auth/tickets/S-0002.md":  tk("S-0002", "story", "Sign in", "1-auth", "draft", "requirements: [R9]\nparent: F-0099\n"),
		"specs/1-auth/tickets/S-0003.md":  "---\nschema: [\n---\n",
		"specs/1-auth/tickets/S-0004.md":  strings.Replace(tk("S-0004", "story", "No id", "1-auth", "draft", ""), "id: S-0004\n", "", 1),
		"specs/2-other/tickets/S-0007.md": tk("S-0007", "story", "Elsewhere", "2-other", "draft", "parent: F-0098\n"),
	})
	approved := render(m, map[string]string{"architecture": "approved"}, true)
	body := section(t, approved, "Problems")
	lines := strings.Split(body, "\n")
	want := []string{
		"- ! R1: must requirement in 1-auth isn't cited by any ticket",
		"- ! README: narrative is empty",
		"- ✗ S-0002: cites R9, which isn't in 1-auth's spec",
		"- ✗ S-0002: parent F-0099 doesn't exist",
		"- ✗ tickets/S-0003.md: ",
		"- ✗ tickets/S-0004.md: ",
	}
	if len(lines) != len(want) {
		t.Fatalf("problems:\n%s", body)
	}
	for i := range want {
		if !strings.HasPrefix(lines[i], want[i]) {
			t.Fatalf("problem %d = %q, want prefix %q\n%s", i, lines[i], want[i], body)
		}
	}
	if strings.Contains(body, "S-0007") || strings.Contains(body, string(filepath.Separator)+"specs") {
		t.Fatalf("another change's problem or a path leaked:\n%s", body)
	}
	if strings.Contains(render(m, map[string]string{"architecture": "draft"}, true), "narrative is empty") ||
		strings.Contains(render(m, map[string]string{"architecture": "approved"}, false), "narrative is empty") {
		t.Fatal("the narrative problem showed when it shouldn't")
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	files := map[string]string{
		"specs/1-auth/tickets/S-0002.md": tk("S-0002", "story", "B", "1-auth", "draft", "depends_on: [S-0003, S-0001]\nexternal:\n  b: 2\n  a: 1\n"),
		"specs/1-auth/tickets/S-0001.md": tk("S-0001", "story", "A", "1-auth", "draft", ""),
		"specs/1-auth/tickets/S-0003.md": tk("S-0003", "story", "C", "1-auth", "draft", ""),
	}
	m := load(t, files)
	first := render(m, nil, false)
	for i := 0; i < 5; i++ {
		if again := render(m, nil, false); again != first {
			t.Fatalf("render %d differs", i)
		}
	}
}
