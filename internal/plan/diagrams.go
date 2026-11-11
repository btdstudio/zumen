package plan

import "strings"

// Diagram is one `###` subsection of an architecture's `## Diagrams` section.
type Diagram struct {
	Heading string // the text after "### ", such as "Flow: approving a gate"
	Line    int    // the heading's line, 1-based
	Source  string // the body of the subsection's first closed mermaid fence, or ""
}

const diagramsSection = "## Diagrams"

// Diagrams reads the `## Diagrams` section of an architecture document.
// section reports whether the heading exists. Each `### ` heading outside
// fences and comments starts a diagram, in file order; text before the first one is
// ignored. Source is the body of the subsection's first fence whose info
// string starts with the word "mermaid", without the fence lines. It's ""
// when there's no such fence, when it's unclosed, or when its body is blank.
func Diagrams(markdown string) (section bool, ds []Diagram) {
	lines := scan(markdown)
	for _, l := range lines {
		if isHeading(l, diagramsSection) {
			section = true
			break
		}
	}
	if !section {
		return false, nil
	}
	cur := -1     // the index of the diagram being read
	seen := false // whether cur's first mermaid fence has been read
	var body []string
	inMermaid := false
	for _, l := range sectionLines(lines, diagramsSection) {
		switch {
		case l.prose() && strings.HasPrefix(l.text, "### "):
			ds = append(ds, Diagram{Heading: strings.TrimSpace(l.text[4:]), Line: l.n})
			cur, seen = len(ds)-1, false
		case cur < 0:
		case l.open:
			inMermaid = !seen && infoWord(l.info) == "mermaid"
			seen = seen || inMermaid
			body = nil
		case l.close:
			if inMermaid {
				if src := strings.Join(body, "\n"); strings.TrimSpace(src) != "" {
					ds[cur].Source = src
				}
			}
			inMermaid = false
		case inMermaid:
			body = append(body, l.text)
		}
	}
	return true, ds
}

// infoWord is the first word of a fence's info string.
func infoWord(info string) string {
	if f := strings.Fields(info); len(f) > 0 {
		return f[0]
	}
	return ""
}
