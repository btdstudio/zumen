package icm

import (
	"fmt"
	"slices"
	"strings"
)

// userOwnedSections are the scaffold's `##` sections that a user fills in, so a
// merge keeps the user's text. Every other scaffold section belongs to zumen.
var userOwnedSections = []string{"Tech stack & standards"}

// retiredSections are `##` headings a past scaffold had and the current one has
// dropped. A merge removes them. There are none yet.
var retiredSections []string

// agentsSection is one `##` section: its heading line through its last non-blank
// line (body), and the blank lines after it (trail). They're kept apart so the
// user's spacing survives a merge.
type agentsSection struct {
	heading string
	body    []string
	trail   []string
}

// agentsDoc is an AGENTS.md split at its `##` lines. The preamble is everything
// before the first one: the `#` title and the intro.
type agentsDoc struct {
	preamble     []string
	sections     []agentsSection
	finalNewline bool
}

// fenceOf reports the fence character and length when line opens or closes a
// fenced code block: up to three spaces, then three or more backticks or tildes.
func fenceOf(line string) (ch byte, n int) {
	rest := strings.TrimLeft(line, " ")
	if len(line)-len(rest) > 3 || len(rest) < 3 || (rest[0] != '`' && rest[0] != '~') {
		return 0, 0
	}
	for n < len(rest) && rest[n] == rest[0] {
		n++
	}
	if n < 3 {
		return 0, 0
	}
	return rest[0], n
}

// parseAgents splits text into its preamble and `##` sections. A `## ` line only
// starts a section outside a code fence, and `###` doesn't.
func parseAgents(text string) agentsDoc {
	doc := agentsDoc{finalNewline: strings.HasSuffix(text, "\n")}
	if text == "" {
		return doc
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	var fenceCh byte
	fenceLen := 0
	var cur *agentsSection
	flush := func() {
		if cur == nil {
			return
		}
		end := len(cur.body)
		for end > 1 && strings.TrimSpace(cur.body[end-1]) == "" {
			end--
		}
		cur.trail = slices.Clone(cur.body[end:])
		cur.body = cur.body[:end]
		doc.sections = append(doc.sections, *cur)
		cur = nil
	}
	for _, line := range lines {
		if fenceCh == 0 && strings.HasPrefix(line, "## ") {
			flush()
			cur = &agentsSection{heading: strings.TrimSpace(strings.TrimRight(line[3:], "\r")), body: []string{line}}
			continue
		}
		if ch, n := fenceOf(line); ch != 0 {
			switch {
			case fenceCh == 0:
				fenceCh, fenceLen = ch, n
			case ch == fenceCh && n >= fenceLen && strings.TrimSpace(strings.TrimLeft(strings.TrimLeft(line, " "), string(ch))) == "":
				fenceCh, fenceLen = 0, 0
			}
		}
		if cur == nil {
			doc.preamble = append(doc.preamble, line)
		} else {
			cur.body = append(cur.body, line)
		}
	}
	flush()
	return doc
}

func (d agentsDoc) String() string {
	var lines []string
	lines = append(lines, d.preamble...)
	for _, s := range d.sections {
		lines = append(lines, s.body...)
		lines = append(lines, s.trail...)
	}
	out := strings.Join(lines, "\n")
	if d.finalNewline {
		out += "\n"
	}
	return out
}

// MergeAgents merges the scaffold's `##` sections into a user's edited
// AGENTS.md. Each zumen-owned section takes the scaffold's text, and a zumen
// section the user's file lacks is inserted after its nearest preceding scaffold
// section. The `#` title, the text under it, the user-owned sections and any
// section of the user's own are kept byte for byte, in the user's order. When
// it can't merge, merged is "" and reason says why: the file has no `##`
// heading, or repeats a zumen heading. The merge is idempotent.
func MergeAgents(user, scaffold string) (merged, reason string) {
	return mergeAgents(user, scaffold, userOwnedSections, retiredSections)
}

func mergeAgents(user, scaffold string, userOwned, retired []string) (merged, reason string) {
	u, s := parseAgents(user), parseAgents(scaffold)
	if len(u.sections) == 0 {
		return "", "no ## headings"
	}
	owned := map[string]bool{}
	scafBy := map[string]agentsSection{}
	for _, sec := range s.sections {
		scafBy[sec.heading] = sec
		if !slices.Contains(userOwned, sec.heading) {
			owned[sec.heading] = true
		}
	}
	seen := map[string]bool{}
	for _, sec := range u.sections {
		if !owned[sec.heading] {
			continue
		}
		if seen[sec.heading] {
			return "", fmt.Sprintf("duplicated heading %q", sec.heading)
		}
		seen[sec.heading] = true
	}

	out := u
	out.sections = nil
	for _, sec := range u.sections {
		switch {
		case slices.Contains(retired, sec.heading):
			continue
		case owned[sec.heading]:
			out.sections = append(out.sections, agentsSection{
				heading: sec.heading,
				body:    slices.Clone(scafBy[sec.heading].body),
				trail:   sec.trail,
			})
		default:
			out.sections = append(out.sections, sec)
		}
	}

	indexOf := func(heading string) int {
		for i, sec := range out.sections {
			if sec.heading == heading {
				return i
			}
		}
		return -1
	}
	for i, sec := range s.sections {
		if !owned[sec.heading] || indexOf(sec.heading) >= 0 {
			continue
		}
		at := 0 // right after the preamble, unless a scaffold section precedes
		for j := i - 1; j >= 0; j-- {
			if k := indexOf(s.sections[j].heading); k >= 0 {
				at = k + 1
				break
			}
		}
		ins := agentsSection{heading: sec.heading, body: slices.Clone(sec.body), trail: slices.Clone(sec.trail)}
		if at == 0 {
			if n := len(out.preamble); n > 0 && strings.TrimSpace(out.preamble[n-1]) != "" {
				out.preamble = append(slices.Clone(out.preamble), "")
			}
		} else if len(out.sections[at-1].trail) == 0 {
			out.sections[at-1].trail = []string{""}
		}
		if at < len(out.sections) && len(ins.trail) == 0 {
			ins.trail = []string{""}
		}
		out.sections = slices.Insert(out.sections, at, ins)
	}
	return out.String(), ""
}
