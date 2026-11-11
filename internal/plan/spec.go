// Package plan is zumen's read model of a change: the requirements in its
// functional spec, the diagrams in its architecture, every ticket in the
// workspace and the ADRs. The gate checks, `tickets check` coverage and the
// overview all read it, so they agree on what's covered or broken.
//
// plan sits below internal/state: it imports internal/adr, internal/settings
// and kiso's ticket and placeholder packages, never state or cli.
package plan

import (
	"fmt"
	"regexp"

	"github.com/btdstudio/kiso/placeholder"
)

// Requirement is one requirement line of a functional spec.
type Requirement struct {
	ID       string // "R3"
	Kind     byte   // 'R' or 'N'
	Priority string // "must", "should", "could", or "" when missing
	Line     int    // 1-based
}

// Severity is a finding's tier: an Error fails a check, a Warning doesn't.
type Severity int

const (
	Error Severity = iota
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Finding is one problem with a change's planning documents. ID names what
// it's about, such as a requirement id.
type Finding struct {
	Severity Severity
	ID       string
	Message  string
}

const specSection = "## Functional requirements"

var (
	requirementRe = regexp.MustCompile(`^\s*[-*]\s+([RN])(\d+)\.\s`)
	priorityRe    = regexp.MustCompile(`— \*\*(must|should|could)\*\*`)
)

// ParseSpec reads the requirements of a functional spec, following ADR-0005.
// Only lines under `## Functional requirements` and outside fenced code count.
// A requirement line is `- R<n>. …` or `- N<n>. …`, and its priority is the
// first `— **must**`, `— **should**` or `— **could**` on that line. A line
// holding a template placeholder isn't a requirement. A missing priority is a
// warning; a repeated id is an error, and only its first occurrence is kept.
// Requirements and findings are in file order.
func ParseSpec(markdown string) ([]Requirement, []Finding) {
	placeholders := map[int]bool{}
	for _, m := range placeholder.Find(markdown) {
		placeholders[m.Line] = true
	}
	var reqs []Requirement
	var findings []Finding
	first := map[string]int{}
	for _, l := range section(scan(markdown), specSection) {
		if l.inFence || placeholders[l.n] {
			continue
		}
		m := requirementRe.FindStringSubmatch(l.text)
		if m == nil {
			continue
		}
		id := m[1] + m[2]
		if prev, ok := first[id]; ok {
			findings = append(findings, Finding{Error, id,
				fmt.Sprintf("line %d: %s is already defined on line %d", l.n, id, prev)})
			continue
		}
		first[id] = l.n
		r := Requirement{ID: id, Kind: m[1][0], Line: l.n}
		if p := priorityRe.FindStringSubmatch(l.text); p != nil {
			r.Priority = p[1]
		} else {
			findings = append(findings, Finding{Warning, id,
				fmt.Sprintf("line %d: %s has no priority (expected — **must**, — **should** or — **could**)", l.n, id)})
		}
		reqs = append(reqs, r)
	}
	return reqs, findings
}
