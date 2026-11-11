package plan

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/btdstudio/kiso/ticket"
)

var adrCitationRe = regexp.MustCompile(`^ADR-(\d{4})$`)

// Problems reports the requirement and ADR findings for every change in the
// workspace, in a stable order: by change, then the spec's own findings, each
// ticket's citation findings by ticket id, uncovered requirements in spec
// order, and ADR citations by ticket id. A ticket belongs to the change whose
// tickets folder holds it, and its citations resolve against that change's
// spec. Callers filter by Change.
func Problems(m *Model) []Finding {
	byChange := map[string][]*ticket.Ticket{}
	for _, t := range m.Tickets {
		c := TicketChange(t)
		byChange[c] = append(byChange[c], t)
	}
	var out []Finding
	for _, cs := range m.Changes {
		tickets := byChange[cs.Name]
		sort.SliceStable(tickets, func(i, j int) bool { return tickets[i].ID < tickets[j].ID })
		add := func(s Severity, id, format string, args ...any) {
			out = append(out, Finding{Severity: s, ID: id, Message: fmt.Sprintf(format, args...), Change: cs.Name})
		}

		for _, f := range cs.Findings {
			add(f.Severity, f.ID, "spec of %s: %s", cs.Name, f.Message)
		}

		defined := map[string]bool{}
		for _, r := range cs.Requirements {
			defined[r.ID] = true
		}
		cited := map[string]bool{}
		for _, t := range tickets {
			for _, id := range t.Requirements {
				cited[id] = true
				switch {
				case !cs.HasSpec:
					add(Error, t.ID, "cites %s, but %s has no spec", id, cs.Name)
				case !defined[id]:
					add(Error, t.ID, "cites %s, which isn't in %s's spec", id, cs.Name)
				}
			}
			if t.Status == ticket.StatusReady && (t.Type == ticket.Story || t.Type == ticket.Task) && len(t.Requirements) == 0 {
				add(Warning, t.ID, "ready %s with no requirements", t.Type)
			}
		}

		if len(tickets) > 0 {
			for _, r := range cs.Requirements {
				if r.Kind == 'R' && r.Priority == "must" && !cited[r.ID] {
					add(Warning, r.ID, "must requirement in %s isn't cited by any ticket", cs.Name)
				}
			}
		}

		for _, t := range tickets {
			for _, a := range t.ADRs {
				mm := adrCitationRe.FindStringSubmatch(a)
				switch {
				case mm == nil:
					add(Warning, t.ID, "adrs entry %q isn't of the form ADR-<nnnn>", a)
				case !hasADR(m, a):
					add(Warning, t.ID, "cites %s, but there's no %s-*.md ADR file", a, mm[1])
				}
			}
		}
	}
	return out
}

func hasADR(m *Model, citation string) bool {
	_, ok := m.ADRs[citation]
	return ok
}

// TicketChange is the change whose folder holds t. Every ticket in a Model was
// loaded from specs/<change>/tickets/, so that's the folder two levels up.
func TicketChange(t *ticket.Ticket) string {
	return filepath.Base(filepath.Dir(filepath.Dir(t.Path)))
}
