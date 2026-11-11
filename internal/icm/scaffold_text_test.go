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
		{"03_spec/personas/analyst.md", "on one line"},
		{"03_spec/templates/functional-spec.md", "all on one line"},
		{"AGENTS.md", "`zumen tickets new <type> <title> [--parent <id>] [--repo <name>] [--requirements <id,...>]`"},
	} {
		data, err := fs.ReadFile(zumen.ScaffoldFS, "scaffold/"+tc.file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), tc.want) {
			t.Errorf("%s lacks %q", tc.file, tc.want)
		}
	}
}
