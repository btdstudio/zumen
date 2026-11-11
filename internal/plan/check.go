package plan

import (
	"path/filepath"
	"strings"

	"github.com/btdstudio/kiso/placeholder"
	"github.com/btdstudio/kiso/repos"
	kstate "github.com/btdstudio/kiso/state"
	"github.com/btdstudio/kiso/ticket"
)

// CheckArtifacts checks a phase's artifacts before a gate transition. It's
// zumen's Machine.Check: spec, architecture and breakdown are checked when
// they're approved, and publish when it's completed. Any other transition
// returns nil without reading anything. A failure to read what it needs is
// reported as a problem, so a gate fails safe rather than passing unchecked.
func CheckArtifacts(root, change, phase, status string) []kstate.Problem {
	switch {
	case phase == "spec" && status == "approved":
		return checkSpec(root, change)
	case phase == "architecture" && status == "approved":
		return checkArchitecture(root, change)
	case phase == "breakdown" && status == "approved":
		return checkBreakdown(root, change)
	case phase == "publish" && status == "complete":
		return checkPublish(root, change)
	}
	return nil
}

// artifact is a change file's root-relative path, with / separators.
func artifact(change, name string) string {
	return specsDir + "/" + change + "/" + name
}

// rel makes a path under root root-relative, with / separators.
func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		path = r
	}
	return filepath.ToSlash(path)
}

func cantCheck(path string, err error) []kstate.Problem {
	return []kstate.Problem{{Path: path, Message: "can't check: " + err.Error()}}
}

// readArtifact reads a change file. When it's missing or unreadable, problems
// says so.
func readArtifact(root, path string) (text string, problems []kstate.Problem) {
	text, ok, err := readIfExists(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return "", cantCheck(path, err)
	}
	if !ok {
		return "", []kstate.Problem{{Path: path, Message: "doesn't exist"}}
	}
	return text, nil
}

func checkSpec(root, change string) []kstate.Problem {
	path := artifact(change, "functional-spec.md")
	text, problems := readArtifact(root, path)
	if problems != nil {
		return problems
	}
	reqs, findings := ParseSpec(text)
	hasR := false
	for _, r := range reqs {
		hasR = hasR || r.Kind == 'R'
	}
	if !hasR {
		problems = append(problems, kstate.Problem{Path: path, Message: "no R requirements under " + specSection})
	}
	for _, f := range findings {
		if f.Severity == Error {
			problems = append(problems, kstate.Problem{Path: path, Message: f.Message})
		}
	}
	return problems
}

func checkArchitecture(root, change string) []kstate.Problem {
	path := artifact(change, "architecture.md")
	text, problems := readArtifact(root, path)
	if problems != nil {
		return problems
	}
	section, ds := Diagrams(text)
	if !section {
		problems = append(problems, kstate.Problem{Path: path, Message: "no " + diagramsSection + " section"})
	} else {
		required := []struct {
			name  string
			match func(heading string) bool
		}{
			{"### Context", func(h string) bool { return h == "Context" }},
			{"### Layers", func(h string) bool { return h == "Layers" }},
			{"### Flow: <name>", isFlow},
		}
		var empty []kstate.Problem
		for _, req := range required {
			found := false
			for _, d := range ds {
				if req.match(d.Heading) {
					found = true
				}
			}
			if !found {
				problems = append(problems, kstate.Problem{Path: path, Message: diagramsSection + " has no " + req.name})
			}
		}
		for _, d := range ds {
			if d.Source != "" {
				continue
			}
			for _, req := range required {
				if req.match(d.Heading) {
					empty = append(empty, kstate.Problem{Path: path, Line: d.Line, Message: "### " + d.Heading + " needs a non-empty mermaid block"})
					break
				}
			}
		}
		problems = append(problems, empty...)
	}
	for _, m := range placeholder.Find(text) {
		problems = append(problems, kstate.Problem{Path: path, Line: m.Line, Message: "unfilled template placeholder " + m.Text})
	}
	return problems
}

// isFlow reports whether a diagram heading is `Flow: <name>` with a name.
func isFlow(heading string) bool {
	name, ok := strings.CutPrefix(heading, "Flow:")
	return ok && strings.TrimSpace(name) != ""
}

// changeTickets loads the model and returns the tickets whose files are in
// the change's own tickets folder, plus that folder's unparsable files.
func changeTickets(root, change string) (*Model, []*ticket.Ticket, []ticket.Problem, error) {
	m, err := Load(root, change)
	if err != nil {
		return nil, nil, nil, err
	}
	dir := filepath.Join(root, specsDir, change, "tickets")
	var mine []*ticket.Ticket
	for _, t := range m.Tickets {
		if filepath.Dir(t.Path) == dir {
			mine = append(mine, t)
		}
	}
	var bad []ticket.Problem
	for _, p := range m.TicketProblems {
		if filepath.Dir(p.ID) == dir {
			bad = append(bad, p)
		}
	}
	return m, mine, bad, nil
}

func checkBreakdown(root, change string) []kstate.Problem {
	folder := artifact(change, "tickets")
	m, mine, bad, err := changeTickets(root, change)
	if err != nil {
		return cantCheck(folder, err)
	}
	reg, err := repos.Load(root, "zumen")
	if err != nil {
		return cantCheck("", err)
	}
	var problems []kstate.Problem
	for _, p := range bad {
		problems = append(problems, kstate.Problem{Path: rel(root, p.ID), Message: p.Message})
	}
	if len(mine) == 0 && len(bad) == 0 {
		return []kstate.Problem{{Path: folder, Message: "no tickets"}}
	}
	byKey := map[string]*ticket.Ticket{}
	for _, t := range mine {
		byKey[t.Path] = t
		if t.ID != "" {
			byKey[t.ID] = t
		}
	}
	for _, p := range ticket.ValidateWorkspace(asDrafts(m.Tickets, byKey), reg.Names()) {
		t, ok := byKey[p.ID]
		if !ok {
			continue
		}
		msg := p.Message
		if p.ID == t.ID {
			msg = t.ID + ": " + msg
		}
		problems = append(problems, kstate.Problem{Path: rel(root, t.Path), Message: msg})
	}
	return problems
}

// asDrafts returns the workspace's tickets with this change's ready tickets
// validated as drafts. Readiness comes after breakdown: a ready ticket's mark
// goes stale exactly when breakdown is reopened to revise it, and it can only
// be re-readied once breakdown is approved again. So the readiness mark doesn't
// block breakdown (tickets check still reports it), while structural problems
// such as a missing parent or an unregistered repo still do.
func asDrafts(all []*ticket.Ticket, mine map[string]*ticket.Ticket) []*ticket.Ticket {
	out := make([]*ticket.Ticket, len(all))
	for i, t := range all {
		out[i] = t
		if mine[t.Path] == t && t.Status == ticket.StatusReady {
			draft := *t
			draft.Status = ticket.StatusDraft
			out[i] = &draft
		}
	}
	return out
}

func checkPublish(root, change string) []kstate.Problem {
	_, mine, _, err := changeTickets(root, change)
	if err != nil {
		return cantCheck(artifact(change, "tickets"), err)
	}
	var problems []kstate.Problem
	for _, t := range mine {
		if (t.Status == ticket.StatusReady || t.Status == ticket.StatusPublished) && t.PublishedHash == "" {
			problems = append(problems, kstate.Problem{Path: rel(root, t.Path),
				Message: t.ID + ": " + t.Status + " but not frozen (no published_hash)"})
		}
	}
	return problems
}
