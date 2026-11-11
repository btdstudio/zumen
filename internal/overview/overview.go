// Package overview renders a change's README overview: a narrative region
// people own and a generated region zumen owns (ADR-0004). Render is pure: the
// CLI gathers the inputs. It reads and writes no files, so the same Input gives
// byte-identical output.
package overview

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/btdstudio/kiso/ticket"
	"github.com/btdstudio/zumen/internal/plan"
)

// Input is everything Render needs.
type Input struct {
	Model          *plan.Model       // plan.Load(root, change)
	Phases         []string          // the machine's phases, in order
	Gates, Reasons map[string]string // the change's record
	Repos          []string          // REPOS.json names, for validation
	ADRDir         string            // the adrDir setting, for links
	NarrativeEmpty bool
}

const none = "_None yet._"

// maxNodes is the largest dependency group drawn as Mermaid; a bigger one is a
// table.
const maxNodes = 25

// Render returns the generated region: the text between the generated markers.
// It starts and ends with a newline.
func Render(in Input) string {
	r := newRenderer(in)
	sections := []struct {
		title string
		body  func() string
	}{
		{"Status", r.status},
		{"Diagrams", r.diagrams},
		{"Work map", r.workMap},
		{"Dependencies", r.dependencies},
		{"Repos", r.repos},
		{"Decisions", r.decisions},
		{"Traceability", r.traceability},
		{"Problems", r.problems},
	}
	var b strings.Builder
	b.WriteString("\n")
	for i, s := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("## " + s.title + "\n\n" + s.body() + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

type renderer struct {
	in      Input
	m       *plan.Model
	mine    []*ticket.Ticket          // the change's tickets, by id
	byID    map[string]*ticket.Ticket // every ticket in the workspace
	inThis  map[string]bool           // ids of the change's tickets
	byPath  map[string]*ticket.Ticket // the change's tickets, by path
	changes map[*ticket.Ticket]string // every ticket's change
}

func newRenderer(in Input) *renderer {
	r := &renderer{in: in, m: in.Model, byID: map[string]*ticket.Ticket{}, inThis: map[string]bool{},
		byPath: map[string]*ticket.Ticket{}, changes: map[*ticket.Ticket]string{}}
	for _, t := range r.m.Tickets {
		c := plan.TicketChange(t)
		r.changes[t] = c
		if _, dup := r.byID[t.ID]; !dup {
			r.byID[t.ID] = t
		}
		if c == r.m.Change {
			r.mine = append(r.mine, t)
			r.inThis[t.ID] = true
			r.byPath[t.Path] = t
		}
	}
	sort.SliceStable(r.mine, func(i, j int) bool { return r.mine[i].ID < r.mine[j].ID })
	return r
}

// cell escapes text for a table cell.
func cell(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

func ticketLink(id string) string {
	return fmt.Sprintf("[%s](tickets/%s.md)", id, id)
}

func (r *renderer) status() string {
	var b strings.Builder
	b.WriteString("| Phase | Status | Note |\n|---|---|---|\n")
	for _, p := range r.in.Phases {
		g := r.in.Gates[p]
		if g == "" {
			g = "—"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", p, g, cell(r.in.Reasons[p]))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (r *renderer) diagrams() string {
	link := "From [architecture.md](architecture.md)."
	if len(r.m.Diagrams) == 0 {
		if r.m.HasArchitecture {
			return none + "\n\n" + link
		}
		return none
	}
	var parts []string
	for _, d := range r.m.Diagrams {
		body := "_No diagram yet._"
		if d.Source != "" {
			body = "```mermaid\n" + d.Source + "\n```"
		}
		parts = append(parts, "### "+d.Heading+"\n\n"+body)
	}
	return strings.Join(parts, "\n\n") + "\n\n" + link
}

// children maps a parent id to the change's tickets under it, by id.
func (r *renderer) children() map[string][]*ticket.Ticket {
	kids := map[string][]*ticket.Ticket{}
	for _, t := range r.mine {
		if t.Parent != "" && r.inThis[t.Parent] {
			kids[t.Parent] = append(kids[t.Parent], t)
		}
	}
	return kids
}

func (r *renderer) workMap() string {
	if len(r.mine) == 0 {
		return none
	}
	kids := r.children()
	seen := map[*ticket.Ticket]bool{}
	var b strings.Builder
	var walk func(t *ticket.Ticket, depth int)
	walk = func(t *ticket.Ticket, depth int) {
		if seen[t] {
			return
		}
		seen[t] = true
		b.WriteString(strings.Repeat("  ", depth) + "- " + r.workLine(t) + "\n")
		for _, k := range kids[t.ID] {
			walk(k, depth+1)
		}
	}
	for _, t := range r.mine {
		if t.Parent == "" || !r.inThis[t.Parent] {
			walk(t, 0)
		}
	}
	// Tickets in a parent cycle are never reached from a root.
	for _, t := range r.mine {
		walk(t, 0)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (r *renderer) workLine(t *ticket.Ticket) string {
	parts := []string{fmt.Sprintf("[**%s**](tickets/%s.md) %s", t.ID, t.ID, t.Title)}
	if t.Repo != "" {
		parts = append(parts, "`"+t.Repo+"`")
	}
	if t.Status != "" {
		parts = append(parts, t.Status)
	}
	if len(t.External) > 0 {
		tools := make([]string, 0, len(t.External))
		for tool := range t.External {
			tools = append(tools, tool)
		}
		sort.Strings(tools)
		var ext []string
		for _, tool := range tools {
			ext = append(ext, tool+" "+t.External[tool])
		}
		parts = append(parts, strings.Join(ext, ", "))
	}
	if len(t.Requirements) > 0 {
		parts = append(parts, strings.Join(t.Requirements, ", "))
	}
	return strings.Join(parts, " · ")
}

// epicOf walks a ticket's parents within the change to its epic, or "".
func (r *renderer) epicOf(t *ticket.Ticket) string {
	seen := map[string]bool{}
	for cur := t; cur != nil && !seen[cur.ID]; {
		seen[cur.ID] = true
		if cur.Type == ticket.Epic {
			return cur.ID
		}
		if cur.Parent == "" || !r.inThis[cur.Parent] {
			return ""
		}
		cur = r.byID[cur.Parent]
	}
	return ""
}

func nodeID(id string) string {
	return strings.ReplaceAll(id, "-", "_")
}

// label is a Mermaid node label: "<id> · <title>", the title cut to 40 runes.
func label(id, title string) string {
	rs := []rune(title)
	if len(rs) > 40 {
		title = strings.TrimRight(string(rs[:39]), " ") + "…"
	}
	return strings.ReplaceAll(id+" · "+title, `"`, "#quot;")
}

func (r *renderer) dependencies() string {
	if len(r.mine) == 0 {
		return none
	}
	groups := map[string][]*ticket.Ticket{}
	for _, t := range r.mine {
		e := r.epicOf(t)
		groups[e] = append(groups[e], t)
	}
	var epics []string
	for e := range groups {
		if e != "" {
			epics = append(epics, e)
		}
	}
	sort.Strings(epics)
	var parts []string
	for _, e := range epics {
		parts = append(parts, "### "+e+" · "+r.byID[e].Title+"\n\n"+r.group(groups[e]))
	}
	if g := groups[""]; len(g) > 0 {
		parts = append(parts, "### No epic\n\n"+r.group(g))
	}
	return strings.Join(parts, "\n\n")
}

// group renders one dependency group as Mermaid, or as a table when it has
// more than maxNodes nodes.
func (r *renderer) group(members []*ticket.Ticket) string {
	in := map[string]bool{}
	for _, t := range members {
		in[t.ID] = true
	}
	type edge struct{ from, to string }
	var edges []edge
	external := map[string]bool{}
	for _, t := range members {
		for _, d := range t.DependsOn {
			edges = append(edges, edge{d, t.ID})
			if !in[d] {
				external[d] = true
			}
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].to != edges[j].to {
			return edges[i].to < edges[j].to
		}
		return edges[i].from < edges[j].from
	})
	var ext []string
	for id := range external {
		ext = append(ext, id)
	}
	sort.Strings(ext)

	if len(members)+len(ext) > maxNodes {
		var b strings.Builder
		b.WriteString("| Ticket | Depends on |\n|---|---|\n")
		for _, t := range members {
			var deps []string
			for _, d := range t.DependsOn {
				deps = append(deps, d+r.otherChange(d))
			}
			fmt.Fprintf(&b, "| %s | %s |\n", ticketLink(t.ID), cell(strings.Join(deps, ", ")))
		}
		return strings.TrimSuffix(b.String(), "\n")
	}

	var b strings.Builder
	b.WriteString("```mermaid\nflowchart LR\n")
	for _, t := range members {
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", nodeID(t.ID), label(t.ID, t.Title))
	}
	for _, id := range ext {
		title := "not found"
		if t, ok := r.byID[id]; ok {
			title = t.Title
		}
		fmt.Fprintf(&b, "  %s[\"%s%s\"]:::external\n", nodeID(id), label(id, title), r.otherChange(id))
	}
	for _, e := range edges {
		fmt.Fprintf(&b, "  %s --> %s\n", nodeID(e.from), nodeID(e.to))
	}
	if len(ext) > 0 {
		b.WriteString("  classDef external stroke-dasharray: 5 5\n")
	}
	b.WriteString("```")
	return b.String()
}

// otherChange is " (<change>)" for a ticket in another change, else "".
func (r *renderer) otherChange(id string) string {
	t, ok := r.byID[id]
	if !ok {
		return ""
	}
	if c := r.changes[t]; c != r.m.Change {
		return " (" + c + ")"
	}
	return ""
}

func (r *renderer) repos() string {
	type row struct {
		counts map[string]int
		ids    []string
	}
	rows := map[string]*row{}
	for _, t := range r.mine {
		if t.Repo == "" {
			continue
		}
		rw := rows[t.Repo]
		if rw == nil {
			rw = &row{counts: map[string]int{}}
			rows[t.Repo] = rw
		}
		rw.counts[t.Status]++
		rw.ids = append(rw.ids, ticketLink(t.ID))
	}
	if len(rows) == 0 {
		return none
	}
	var names []string
	for n := range rows {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("| Repo | draft | ready | published | Tickets |\n|---|---|---|---|---|\n")
	for _, n := range names {
		rw := rows[n]
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %s |\n", n, rw.counts[ticket.StatusDraft], rw.counts[ticket.StatusReady],
			rw.counts[ticket.StatusPublished], strings.Join(rw.ids, ", "))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (r *renderer) decisions() string {
	citedBy := map[string][]string{}
	for _, t := range r.mine {
		for _, a := range t.ADRs {
			if ids := citedBy[a]; len(ids) == 0 || ids[len(ids)-1] != t.ID {
				citedBy[a] = append(citedBy[a], t.ID)
			}
		}
	}
	if len(citedBy) == 0 {
		return none
	}
	var cites []string
	for a := range citedBy {
		cites = append(cites, a)
	}
	sort.Strings(cites)
	var b strings.Builder
	b.WriteString("| ADR | Title | Status | Cited by |\n|---|---|---|---|\n")
	for _, a := range cites {
		name, title, status := cell(a), "not found", ""
		if rec, ok := r.m.ADRs[a]; ok {
			name = fmt.Sprintf("[%s](../../%s/%s)", a, filepath.ToSlash(r.in.ADRDir), filepath.Base(rec.Path))
			title, status = cell(rec.Title), cell(rec.Status)
		}
		var by []string
		for _, id := range citedBy[a] {
			by = append(by, ticketLink(id))
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", name, title, status, strings.Join(by, ", "))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (r *renderer) traceability() string {
	if !r.m.HasSpec || len(r.m.Requirements) == 0 {
		return none
	}
	covered := map[string][]string{}
	for _, t := range r.mine {
		for _, id := range t.Requirements {
			if ids := covered[id]; len(ids) == 0 || ids[len(ids)-1] != t.ID {
				covered[id] = append(covered[id], t.ID)
			}
		}
	}
	var b strings.Builder
	b.WriteString("From [functional-spec.md](functional-spec.md).\n\n| Req | Priority | Covered by |\n|---|---|---|\n")
	for _, req := range r.m.Requirements {
		p := req.Priority
		if p == "" {
			p = "—"
		}
		var links []string
		for _, id := range covered[req.ID] {
			links = append(links, ticketLink(id))
		}
		by := strings.Join(links, ", ")
		if by == "" && req.Kind == 'R' && req.Priority == "must" {
			by = "**uncovered**"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", req.ID, p, by)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// problemID is a ticket problem's id as shown: the ticket id, or the file
// within the change folder when kiso keyed it by path, so no root-dependent
// path reaches the output.
func (r *renderer) problemID(id string) string {
	if strings.HasSuffix(id, ".md") {
		return "tickets/" + filepath.Base(id)
	}
	return id
}

func (r *renderer) problems() string {
	type item struct{ mark, id, msg string }
	var items []item
	for _, p := range ticket.ValidateWorkspace(r.m.Tickets, r.in.Repos) {
		if r.inThis[p.ID] || r.byPath[p.ID] != nil {
			items = append(items, item{"✗", r.problemID(p.ID), p.Message})
		}
	}
	for _, p := range r.m.TicketProblems {
		dir := filepath.Dir(p.ID)
		if filepath.Base(dir) == "tickets" && filepath.Base(filepath.Dir(dir)) == r.m.Change {
			items = append(items, item{"✗", r.problemID(p.ID), p.Message})
		}
	}
	for _, f := range plan.Problems(r.m) {
		if f.Change != r.m.Change {
			continue
		}
		mark := "✗"
		if f.Severity == plan.Warning {
			mark = "!"
		}
		items = append(items, item{mark, f.ID, f.Message})
	}
	if r.in.Gates["architecture"] == "approved" && r.in.NarrativeEmpty {
		items = append(items, item{"!", "README", "narrative is empty"})
	}
	if len(items) == 0 {
		return none
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].id != items[j].id {
			return items[i].id < items[j].id
		}
		return items[i].msg < items[j].msg
	})
	var b strings.Builder
	for _, it := range items {
		id := it.id
		if r.inThis[id] {
			id = ticketLink(id)
		}
		fmt.Fprintf(&b, "- %s %s: %s\n", it.mark, id, it.msg)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
