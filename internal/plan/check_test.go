package plan

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	kstate "github.com/btdstudio/kiso/state"
)

const validSpec = "# Functional spec — Probe\n\n## Functional requirements\n\n- R1. One. — **must**\n- N1. Two. — **must**\n"

const validArchitecture = `# Architecture — Probe

## Overview

The shape.

## Diagrams

### Context

` + "```mermaid\nflowchart LR\n  a --> b\n```" + `

### Layers

` + "```mermaid\nflowchart TB\n  c --> d\n```" + `

### Flow: approving

` + "```mermaid\nsequenceDiagram\n  A->>B: go\n```" + `

## Notes

- None.
`

// checkRoot is a workspace with change 1-probe active, an "app" repo, and no
// artifacts.
func checkRoot(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "specs", "1-probe"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "REPOS.json"), `{"repos": {"app": {"remote": "https://github.com/o/app", "defaultBranch": "main"}}}`)
	return root
}

func storyText(id, change, repo, status, extra string) string {
	return "---\nschema: 1\nid: " + id + "\ntype: story\ntitle: Story " + id + "\nrepo: " + repo +
		"\nchange: " + change + "\nstatus: " + status + "\n" + extra + "---\n\n## Context\nSomething.\n\n## Acceptance criteria\n- [ ] AC1: it works\n"
}

func strs(ps []kstate.Problem) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.String())
	}
	return out
}

func wantProblems(t *testing.T, got []kstate.Problem, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(strs(got), want) {
		t.Fatalf("problems:\n  %s\nwant:\n  %s", strings.Join(strs(got), "\n  "), strings.Join(want, "\n  "))
	}
}

const (
	specPath = "specs/1-probe/functional-spec.md"
	archPath = "specs/1-probe/architecture.md"
)

// AC8: only approved spec/architecture/breakdown and complete publish check.
func TestCheckArtifactsOnlyOnCheckedTransitions(t *testing.T) {
	root := checkRoot(t)
	for _, c := range [][2]string{
		{"spec", "await-approval"}, {"spec", "draft"}, {"architecture", "await-approval"},
		{"breakdown", "draft"}, {"publish", "in-progress"}, {"publish", "approved"},
		{"research", "skipped"}, {"intake", "approved"}, {"nonsense", "approved"},
	} {
		if ps := CheckArtifacts(root, "1-probe", c[0], c[1]); ps != nil {
			t.Errorf("%s %s = %v", c[0], c[1], strs(ps))
		}
	}
	for _, c := range [][2]string{{"spec", "approved"}, {"architecture", "approved"}, {"breakdown", "approved"}} {
		if ps := CheckArtifacts(root, "1-probe", c[0], c[1]); len(ps) == 0 {
			t.Errorf("%s %s found nothing in an empty change", c[0], c[1])
		}
	}
}

// AC5.
func TestCheckSpec(t *testing.T) {
	cases := []struct {
		name string
		spec string // "" means no file
		want []string
	}{
		{"missing", "", []string{specPath + ": doesn't exist"}},
		{"valid", validSpec, nil},
		{"template only",
			"# Functional spec — <name>\n\n## Functional requirements\n\n- R1. <capability, verifiable> — **must | should | could**\n",
			[]string{specPath + ": no R requirements under ## Functional requirements"}},
		{"only N requirements",
			"## Functional requirements\n\n- N1. Fast. — **must**\n",
			[]string{specPath + ": no R requirements under ## Functional requirements"}},
		{"misspelled heading",
			"## Functional Requirements\n\n- R1. One. — **must**\n",
			[]string{specPath + ": no R requirements under ## Functional requirements"}},
		{"duplicate id",
			"## Functional requirements\n\n- R1. One. — **must**\n- R1. Again. — **must**\n",
			[]string{specPath + ": line 4: R1 is already defined on line 3"}},
		{"missing priority only warns",
			"## Functional requirements\n\n- R1. No priority.\n",
			nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := checkRoot(t)
			if c.spec != "" {
				write(t, filepath.Join(root, specPath), c.spec)
			}
			wantProblems(t, CheckArtifacts(root, "1-probe", "spec", "approved"), c.want...)
		})
	}
}

// AC1, AC3, AC4.
func TestCheckArchitecture(t *testing.T) {
	flowless := strings.Replace(validArchitecture, "### Flow: approving", "### Notes on flow", 1)
	cases := []struct {
		name string
		arch string // "" means no file
		want []string
	}{
		{"missing", "", []string{archPath + ": doesn't exist"}},
		{"valid", validArchitecture, nil},
		{"no Diagrams section (AC1)",
			"# Architecture — Probe\n\n## Overview\n\nText.\n",
			[]string{archPath + ": no ## Diagrams section"}},
		{"no Flow (AC3)", flowless, []string{archPath + ": ## Diagrams has no ### Flow: <name>"}},
		{"Flow without a name",
			strings.Replace(validArchitecture, "### Flow: approving", "### Flow:  ", 1),
			[]string{archPath + ": ## Diagrams has no ### Flow: <name>"}},
		{"no Context and Layers",
			"## Diagrams\n\n### Flow: x\n```mermaid\na\n```\n",
			[]string{archPath + ": ## Diagrams has no ### Context", archPath + ": ## Diagrams has no ### Layers"}},
		{"empty Flow block (AC3)",
			strings.Replace(validArchitecture, "sequenceDiagram\n  A->>B: go\n", "", 1),
			[]string{archPath + ":23: ### Flow: approving needs a non-empty mermaid block"}},
		{"Layers with no block",
			strings.Replace(validArchitecture, "```mermaid\nflowchart TB\n  c --> d\n```", "Prose only.", 1),
			[]string{archPath + ":16: ### Layers needs a non-empty mermaid block"}},
		{"optional subsections aren't checked",
			strings.Replace(validArchitecture, "## Notes", "### Deployment\n\nLater.\n\n## Notes", 1),
			nil},
		{"placeholder (AC4)",
			strings.Replace(validArchitecture, "The shape.", "Talks to <component>.", 1),
			[]string{archPath + ":5: unfilled template placeholder <component>"}},
		{"placeholders are reported without Diagrams too",
			"# Architecture — <short name>\n\nText.\n",
			[]string{archPath + ": no ## Diagrams section", archPath + ":1: unfilled template placeholder <short name>"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := checkRoot(t)
			if c.arch != "" {
				write(t, filepath.Join(root, archPath), c.arch)
			}
			wantProblems(t, CheckArtifacts(root, "1-probe", "architecture", "approved"), c.want...)
		})
	}
}

// AC6.
func TestCheckBreakdown(t *testing.T) {
	tickets := filepath.Join("specs", "1-probe", "tickets")
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"no tickets", nil, []string{"specs/1-probe/tickets: no tickets"}},
		{"valid ticket", map[string]string{
			filepath.Join(tickets, "S-0001.md"): storyText("S-0001", "1-probe", "app", "draft", ""),
		}, nil},
		{"unregistered repo", map[string]string{
			filepath.Join(tickets, "S-0001.md"): storyText("S-0001", "1-probe", "ghost", "draft", ""),
		}, []string{"specs/1-probe/tickets/S-0001.md: S-0001: repo \"ghost\" isn't registered"}},
		{"another change's bad ticket is ignored", map[string]string{
			filepath.Join(tickets, "S-0001.md"):                       storyText("S-0001", "1-probe", "app", "draft", ""),
			filepath.Join("specs", "2-other", "tickets", "S-0005.md"): storyText("S-0005", "2-other", "ghost", "draft", ""),
		}, nil},
		{"a dependency in another change resolves", map[string]string{
			filepath.Join(tickets, "S-0001.md"):                       storyText("S-0001", "1-probe", "app", "draft", "depends_on: [S-0005]\n"),
			filepath.Join("specs", "2-other", "tickets", "S-0005.md"): storyText("S-0005", "2-other", "app", "draft", ""),
		}, nil},
		{"a missing dependency counts against this change", map[string]string{
			filepath.Join(tickets, "S-0001.md"): storyText("S-0001", "1-probe", "app", "draft", "depends_on: [S-0009]\n"),
		}, []string{"specs/1-probe/tickets/S-0001.md: S-0001: depends_on S-0009 doesn't exist"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := checkRoot(t)
			for p, text := range c.files {
				write(t, filepath.Join(root, p), text)
			}
			wantProblems(t, CheckArtifacts(root, "1-probe", "breakdown", "approved"), c.want...)
		})
	}
}

// An unparsable file in the change's folder is a problem, and counts as a
// ticket for "no tickets".
func TestCheckBreakdownUnparsable(t *testing.T) {
	root := checkRoot(t)
	write(t, filepath.Join(root, "specs", "1-probe", "tickets", "S-0002.md"), "---\nschema: [\n---\n")
	got := CheckArtifacts(root, "1-probe", "breakdown", "approved")
	if len(got) != 1 || got[0].Path != "specs/1-probe/tickets/S-0002.md" || got[0].Line != 0 || got[0].Message == "" {
		t.Fatalf("problems = %v", strs(got))
	}
}

// AC7.
func TestCheckPublish(t *testing.T) {
	root := checkRoot(t)
	dir := filepath.Join(root, "specs", "1-probe", "tickets")
	write(t, filepath.Join(dir, "S-0001.md"), storyText("S-0001", "1-probe", "app", "ready", ""))
	write(t, filepath.Join(dir, "S-0002.md"), storyText("S-0002", "1-probe", "app", "published", "published_hash: \"v2:abc\"\n"))
	write(t, filepath.Join(dir, "S-0003.md"), storyText("S-0003", "1-probe", "app", "draft", ""))
	write(t, filepath.Join(dir, "S-0004.md"), storyText("S-0004", "1-probe", "app", "published", ""))
	write(t, filepath.Join(root, "specs", "2-other", "tickets", "S-0005.md"), storyText("S-0005", "2-other", "app", "ready", ""))
	wantProblems(t, CheckArtifacts(root, "1-probe", "publish", "complete"),
		"specs/1-probe/tickets/S-0001.md: S-0001: ready but not frozen (no published_hash)",
		"specs/1-probe/tickets/S-0004.md: S-0004: published but not frozen (no published_hash)")
}

// D2: a failure to check is a problem, not a pass.
func TestCheckArtifactsFailsSafe(t *testing.T) {
	root := checkRoot(t)
	if err := os.MkdirAll(filepath.Join(root, specPath), 0o755); err != nil {
		t.Fatal(err)
	}
	got := CheckArtifacts(root, "1-probe", "spec", "approved")
	if len(got) != 1 || got[0].Path != specPath || !strings.HasPrefix(got[0].Message, "can't check: ") {
		t.Fatalf("unreadable spec = %v", strs(got))
	}

	root = checkRoot(t)
	write(t, filepath.Join(root, "REPOS.json"), "{")
	write(t, filepath.Join(root, "specs", "1-probe", "tickets", "S-0001.md"), storyText("S-0001", "1-probe", "app", "draft", ""))
	got = CheckArtifacts(root, "1-probe", "breakdown", "approved")
	if len(got) != 1 || !strings.HasPrefix(got[0].Message, "can't check: ") {
		t.Fatalf("bad REPOS.json = %v", strs(got))
	}

	root = checkRoot(t)
	write(t, filepath.Join(root, ".zumen", "settings.json"), "{")
	got = CheckArtifacts(root, "1-probe", "publish", "complete")
	if len(got) != 1 || !strings.HasPrefix(got[0].Message, "can't check: ") {
		t.Fatalf("bad settings = %v", strs(got))
	}
}
