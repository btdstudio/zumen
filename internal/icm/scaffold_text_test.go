package icm

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/zumen"
)

// Scaffold text for diagrams, the narrative and requirement citations (S-0030).

// AC3: a fresh workspace's architecture template has ## Diagrams right after
// the Overview section, and each required diagram precedes a mermaid fence.
func TestScaffoldArchitectureTemplateDiagrams(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir, Options{NoGit: true}, "none"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "04_architecture", "templates", "architecture.md"))
	if err != nil {
		t.Fatal(err)
	}
	var h2 []string
	lines := strings.Split(string(data), "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "## ") {
			h2 = append(h2, l)
		}
	}
	if len(h2) < 2 || h2[0] != "## Overview" || h2[1] != "## Diagrams" {
		t.Fatalf("sections = %q, want ## Overview then ## Diagrams", h2)
	}
	for _, heading := range []string{"### Context", "### Layers", "### Flow:"} {
		found := false
		for i, l := range lines {
			if !strings.HasPrefix(l, heading) {
				continue
			}
			for _, next := range lines[i+1:] {
				if strings.HasPrefix(next, "#") {
					break
				}
				if next == "```mermaid" {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("%s has no mermaid fence", heading)
		}
	}
}

// AC5: the personas, contexts, spec template and AGENTS.md carry the new
// guidance.
func TestScaffoldGuidanceText(t *testing.T) {
	for _, tc := range []struct{ file, want string }{
		{"04_architecture/templates/architecture.md", "Use standard `mermaid`"},
		{"04_architecture/templates/architecture.md", "about 15 nodes per diagram"},
		{"04_architecture/templates/architecture.md", "Every repo in the change appears in Layers"},
		{"04_architecture/templates/architecture.md", "in backticks"},
		{"04_architecture/personas/architect.md", "Draw the `## Diagrams`"},
		{"04_architecture/context.md", "`<!-- zumen:narrative -->`"},
		{"04_architecture/context.md", "about 250 words"},
		{"04_architecture/context.md", "run `zumen overview` first"},
		{"05_breakdown/context.md", "refresh the narrative"},
		{"05_breakdown/personas/architect.md", "[--requirements <id,...>]"},
		{"05_breakdown/personas/architect.md", "cites the requirements it satisfies"},
		{"05_breakdown/personas/architect.md", "stories and tasks with no parent"},
		{"05_breakdown/personas/architect.md", "one reviewable PR"},
		{"05_breakdown/personas/architect.md", "too large to review"},
		{"05_breakdown/personas/architect.md", "ship at different times"},
		{"05_breakdown/personas/architect.md", "A repo's only story drops the template's split-reason line"},
		{"05_breakdown/personas/architect.md", "more than 8 stories"},
		{"05_breakdown/personas/architect.md", "Never add an epic or feature to group one"},
		{"05_breakdown/personas/architect.md", "There are no release tickets"},
		{"05_breakdown/personas/architect.md", "its last story"},
		{"05_breakdown/personas/architect.md", "`tomo issues new`"},
		{"05_breakdown/personas/architect.md", "Tickets, not file-scoped tasks"},
		{"05_breakdown/personas/architect.md", "Never set `status: ready`"},
		{"05_breakdown/personas/architect.md", "publish target groups tickets"},
		{"05_breakdown/personas/architect.md", "Add an epic only above two or more features"},
		{"05_breakdown/personas/architect.md", "it isn't a target"},
		{"05_breakdown/personas/architect.md", "Use a task for work in one repo"},
		{"05_breakdown/personas/architect.md", "`depends_on` the last story of the repo it bumps"},
		{"05_breakdown/personas/architect.md", "A story lands in one repo"},
		{"05_breakdown/templates/ticket.md", "too large to review as one PR"},
		{"05_breakdown/context.md", "features and epics only where grouping helps"},
		{"AGENTS.md", "features and epics only where grouping helps"},
		{"AGENTS.md", "Edits inside zumen's `##` sections are overwritten by `zumen update`"},
		{"AGENTS.md", "Any phase may also open a file listed in `references/index.md`"},
		{"AGENTS.md", "references/ # reference files + your own index.md (zumen never edits it)"},
		{"references/context.md", "`purpose`, `from` and `added` are required"},
		{"references/context.md", "`summary` is optional, but required when an agent may not be able to read the format"},
		{"references/context.md", "start it with `AI chat`"},
		{"references/context.md", "`index.md` is yours: zumen never creates, edits or overwrites it"},
		{"references/context.md", "`zumen references check` warns about"},
		{"06_publish/sync.md", "only for a ticket that has a parent"},
		{"06_publish/sync-examples/github.md", "needed only when the change has epics or features"},
		{"06_publish/sync.md", "Its parent (if it has one)"},
		{"06_publish/sync-examples/github.md", "Its parent (if it has one)"},
		{"06_publish/sync-examples/github.md", "Only for a ticket that has a parent"},
		{"06_publish/sync-examples/github.md", "needs no planning repo"},
		{"03_spec/personas/analyst.md", "on one line"},
		{"03_spec/templates/functional-spec.md", "all on one line"},
		{"AGENTS.md", "`zumen tickets new <type> <title> [--parent <id>] [--repo <name>] [--depends <id,id>] [--requirements <id,id>]`"},
	} {
		data, err := fs.ReadFile(zumen.ScaffoldFS, "scaffold/"+tc.file)
		if err != nil {
			t.Fatal(err)
		}
		// Whitespace is collapsed, so rewrapping a paragraph can't hide a phrase.
		if !strings.Contains(strings.Join(strings.Fields(string(data)), " "), tc.want) {
			t.Errorf("%s lacks %q", tc.file, tc.want)
		}
	}
}

// AC4: no description of breakdown, in the scaffold or the README, says it always
// produces epics and features. Whitespace is collapsed, so a rewrap can't hide it.
func TestBreakdownWordingIsNotAlwaysEpics(t *testing.T) {
	files := map[string]string{}
	sub, err := fs.Sub(zumen.ScaffoldFS, "scaffold")
	if err != nil {
		t.Fatal(err)
	}
	err = fs.WalkDir(sub, ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(rel, ".md") {
			return err
		}
		data, err := fs.ReadFile(sub, rel)
		files["scaffold/"+rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	files["README.md"] = string(readme)

	for name, text := range files {
		flat := strings.Join(strings.Fields(text), " ")
		for _, old := range []string{"epics, features, stories and tasks", "epics, then features"} {
			if strings.Contains(flat, old) {
				t.Errorf("%s still says %q", name, old)
			}
		}
	}
	if !strings.Contains(files["README.md"], "features and epics only where grouping helps") {
		t.Error("README.md doesn't say features and epics are only for where grouping helps")
	}
}

// AC4: every routing row can read the references index.
func TestRoutingRowsReadTheReferencesIndex(t *testing.T) {
	rows := 0
	inTable := false
	for _, line := range strings.Split(scaffoldText(t, "AGENTS.md"), "\n") {
		switch {
		case strings.HasPrefix(line, "| Task / Intent"):
			inTable = true
		case inTable && strings.HasPrefix(line, "|---"):
		case inTable && strings.HasPrefix(line, "| "):
			rows++
			cells := strings.Split(line, "|")
			if len(cells) != 7 || !strings.Contains(cells[4], "`references/index.md`") {
				t.Errorf("a routing row's Read column lacks references/index.md: %s", line)
			}
		default:
			inTable = false
		}
	}
	if rows < 8 {
		t.Errorf("found %d routing rows, want at least 8", rows)
	}
}

// N3: the references guide names no third-party tool.
func TestReferencesGuideNamesNoTools(t *testing.T) {
	text := strings.ToLower(scaffoldText(t, "references/context.md"))
	for _, tool := range []string{"obsidian", "notion", "confluence", "chatgpt", "claude", "gemini", "copilot", "slack", "github", "google", "onenote", "evernote"} {
		if strings.Contains(text, tool) {
			t.Errorf("references/context.md names %q", tool)
		}
	}
}

func scaffoldText(t *testing.T, rel string) string {
	t.Helper()
	data, err := fs.ReadFile(zumen.ScaffoldFS, "scaffold/"+rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// AC10: the README's workspace tree lists the references room.
func TestReadmeListsTheReferencesRoom(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.Join(strings.Fields(string(readme)), " ")
	for _, want := range []string{
		"references/ reference files + your own index.md (zumen never edits it)",
		"`zumen references check`",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README.md lacks %q", want)
		}
	}
	if strings.Contains(strings.ToLower(flat), "ideas") {
		t.Error("README.md mentions ideas, which belong to a later story")
	}
}
