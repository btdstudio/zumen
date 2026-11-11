package plan

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// AC6: the synthetic fixture mirrors a real architecture's layout.
func TestDiagramsFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/architecture.md")
	if err != nil {
		t.Fatal(err)
	}
	section, ds := Diagrams(string(data))
	if !section {
		t.Fatal("section = false")
	}
	want := []string{"Context", "Layers", "Flow: approving a gate", "Flow: regenerating the overview", "Flow: tomo PR link"}
	lines := strings.Split(string(data), "\n")
	var got []string
	for _, d := range ds {
		got = append(got, d.Heading)
		if lines[d.Line-1] != "### "+d.Heading {
			t.Errorf("%s: line %d = %q", d.Heading, d.Line, lines[d.Line-1])
		}
		if d.Source == "" {
			t.Errorf("%s: empty source", d.Heading)
		}
		if strings.Contains(d.Source, "```") || strings.Contains(d.Source, "notcounted") || strings.Contains(d.Source, "plain text") {
			t.Errorf("%s: source = %q", d.Heading, d.Source)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("headings = %q\nwant       %q", got, want)
	}
	if want := "flowchart TB\n  cli --> state\n  state --> plan\n  plan --> adr"; ds[1].Source != want {
		t.Fatalf("Layers source = %q, want %q", ds[1].Source, want)
	}
}

func TestDiagrams(t *testing.T) {
	head := "# Architecture — X\n\n## Diagrams\n\n" // body starts on line 5
	cases := []struct {
		name    string
		doc     string // the whole document, instead of head + body
		body    string
		section bool
		want    []Diagram
	}{
		{
			name: "no section (AC7)",
			doc:  "# Architecture\n\n## Overview\n\n### Context\n```mermaid\na\n```\n",
		},
		{
			name:    "section without subsections",
			body:    "Some text.\n",
			section: true,
		},
		{
			name:    "no fence and an empty fence (AC7)",
			body:    "### A\nNo diagram yet.\n### B\n```mermaid\n```\n### C\n```mermaid\n   \n```\n",
			section: true,
			want:    []Diagram{{"A", 5, ""}, {"B", 7, ""}, {"C", 10, ""}},
		},
		{
			name:    "a non-mermaid fence only",
			body:    "### A\n```text\nflowchart LR\n```\n",
			section: true,
			want:    []Diagram{{"A", 5, ""}},
		},
		{
			name:    "an unclosed mermaid fence",
			body:    "### A\n```mermaid\nflowchart LR\n",
			section: true,
			want:    []Diagram{{"A", 5, ""}},
		},
		{
			name:    "only the first mermaid fence counts, even when empty",
			body:    "### A\n```mermaid\n```\n```mermaid\nflowchart LR\n```\n### B\n```mermaid\nfirst\n```\n```mermaid\nsecond\n```\n",
			section: true,
			want:    []Diagram{{"A", 5, ""}, {"B", 11, "first"}},
		},
		{
			name:    "tildes, an info string after mermaid, indentation kept",
			body:    "### A\n~~~ mermaid title\n  a --> b\n~~~\n",
			section: true,
			want:    []Diagram{{"A", 5, "  a --> b"}},
		},
		{
			name:    "mermaid must be the info string's first word",
			body:    "### A\n```mermaidish\nx\n```\n```text mermaid\ny\n```\n",
			section: true,
			want:    []Diagram{{"A", 5, ""}},
		},
		{
			name:    "fences before the first subsection are ignored",
			body:    "```mermaid\nintro\n```\n### A\n```mermaid\nreal\n```\n",
			section: true,
			want:    []Diagram{{"A", 8, "real"}},
		},
		{
			name:    "a heading in a fence isn't a subsection",
			body:    "### A\n```mermaid\n### not a heading\n## nor this\n```\n",
			section: true,
			want:    []Diagram{{"A", 5, "### not a heading\n## nor this"}},
		},
		{
			name:    "a commented-out subsection and fence don't count",
			body:    "<!--\n### Old\n```mermaid\nold\n```\n-->\n### A\n```mermaid\nnew\n```\n",
			section: true,
			want:    []Diagram{{"A", 11, "new"}},
		},
		{
			name:    "the next ## ends the section",
			body:    "### A\n```mermaid\na\n```\n## Context\n### Not a diagram\n```mermaid\nb\n```\n",
			section: true,
			want:    []Diagram{{"A", 5, "a"}},
		},
		{
			name:    "headings are trimmed, crlf",
			body:    "###   Flow: x  \r\n```mermaid\r\na\r\n```\r\n",
			section: true,
			want:    []Diagram{{"Flow: x", 5, "a"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := c.doc
			if doc == "" {
				doc = head + c.body
			}
			section, ds := Diagrams(doc)
			if section != c.section {
				t.Errorf("section = %v, want %v", section, c.section)
			}
			if !reflect.DeepEqual(ds, c.want) {
				t.Errorf("diagrams = %+v\nwant       %+v", ds, c.want)
			}
		})
	}
}

func TestDiagramsHeadingInFence(t *testing.T) {
	section, ds := Diagrams("```\n## Diagrams\n### A\n```\n")
	if section || ds != nil {
		t.Fatalf("section = %v, ds = %+v", section, ds)
	}
}
