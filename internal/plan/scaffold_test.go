package plan

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/zumen"
)

// The scaffold's architecture template, run through the real architecture
// check (S-0030).

func templateArchitecture(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(zumen.ScaffoldFS, "scaffold/04_architecture/templates/architecture.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// diagramsLines is the 1-based line range of the ## Diagrams section.
func diagramsLines(t *testing.T, text string) (first, last int) {
	t.Helper()
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		switch {
		case l == "## Diagrams":
			first = i + 1
		case first > 0 && strings.HasPrefix(l, "## "):
			return first, i
		}
	}
	t.Fatal("no ## Diagrams section")
	return 0, 0
}

// checkTemplate writes text as change x's architecture.md and returns the
// problems on lines inside ## Diagrams.
func checkTemplate(t *testing.T, text string) []string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "specs", "x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "architecture.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	first, last := diagramsLines(t, text)
	var in []string
	for _, p := range checkArchitecture(root, "x") {
		if p.Line == 0 || (p.Line >= first && p.Line <= last) {
			in = append(in, p.Message)
		}
	}
	return in
}

// AC1: the unedited template fails, with a placeholder problem for each
// diagram's slot line and for the Flow heading.
func TestTemplateArchitectureUnfilledFails(t *testing.T) {
	got := strings.Join(checkTemplate(t, templateArchitecture(t)), "\n")
	for _, want := range []string{
		"unfilled template placeholder <The system, its users and the external systems it talks to.>",
		"unfilled template placeholder <Each repo in the change and the layers inside it. Every repo appears here.>",
		"unfilled template placeholder <name of the main flow>",
		"unfilled template placeholder <The steps of this flow, across the layers above.>",
		"### Context needs a non-empty mermaid block",
		"### Layers needs a non-empty mermaid block",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// AC2: with each slot line replaced, the Flow named and each fence filled, the
// diagrams raise no problem. Placeholders elsewhere in the template still do.
func TestTemplateArchitectureFilledPasses(t *testing.T) {
	text := templateArchitecture(t)
	first, last := diagramsLines(t, text)
	lines := strings.Split(text, "\n")
	var out []string
	for i, l := range lines {
		n := i + 1
		inSection := n > first && n < last
		switch {
		case inSection && strings.HasPrefix(l, "### Flow:"):
			out = append(out, "### Flow: approving a gate")
		case inSection && strings.HasPrefix(l, "<") && strings.HasSuffix(l, ">"):
			out = append(out, "What the diagram shows.")
		case inSection && l == "```mermaid":
			out = append(out, l, "flowchart LR", "  a --> b")
		default:
			out = append(out, l)
		}
	}
	if problems := checkTemplate(t, strings.Join(out, "\n")); len(problems) != 0 {
		t.Fatalf("filled diagrams still report:\n%s", strings.Join(problems, "\n"))
	}
}
