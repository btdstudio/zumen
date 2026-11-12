package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/btdstudio/kiso/repos"
	"github.com/btdstudio/kiso/ticket"
	"github.com/btdstudio/zumen"
	"github.com/btdstudio/zumen/internal/plan"
	"github.com/btdstudio/zumen/internal/state"
)

func runTickets(root string, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usageText("tickets"))
		return 1
	}
	var code int
	var err error
	switch args[0] {
	case "new":
		code, err = ticketsNew(root, args[1:])
	case "check":
		code, err = ticketsCheck(root)
	case "status":
		code, err = ticketsStatus(root)
	case "order":
		code, err = ticketsOrder(root)
	case "ready":
		code, err = ticketsReady(root, args[1:])
	case "mark-published":
		code, err = ticketsMarkPublished(root, args[1:])
	case "unfreeze":
		code, err = ticketsUnfreeze(root, args[1:])
	default:
		fmt.Fprint(os.Stderr, usageText("tickets"))
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return code
}

func ticketDir(root, change string) string {
	return filepath.Join(state.ChangeDir(root, change), "tickets")
}

// loadTickets reads every ticket in the workspace, across all changes.
func loadTickets(root string) ([]*ticket.Ticket, []ticket.Problem, error) {
	changes, err := state.KnownChanges(root)
	if err != nil {
		return nil, nil, err
	}
	var all []*ticket.Ticket
	var problems []ticket.Problem
	for _, c := range changes {
		tickets, bad, err := ticket.LoadDir(ticketDir(root, c.Name))
		if err != nil {
			return nil, nil, err
		}
		all = append(all, tickets...)
		problems = append(problems, bad...)
	}
	return all, problems, nil
}

// templateBody is the ticket body from the embedded template, the one source of the
// ticket format.
func templateBody() (string, error) {
	data, err := fs.ReadFile(zumen.ScaffoldFS, "scaffold/05_breakdown/templates/ticket.md")
	if err != nil {
		return "", err
	}
	t, err := ticket.Parse(data)
	if err != nil {
		return "", err
	}
	return t.Body, nil
}

func ticketsNew(root string, args []string) (int, error) {
	var pos []string
	f := ticket.Ticket{}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--parent" && i+1 < len(args):
			i++
			f.Parent = args[i]
		case a == "--repo" && i+1 < len(args):
			i++
			f.Repo = args[i]
		case a == "--depends" && i+1 < len(args):
			i++
			f.DependsOn = strings.Split(args[i], ",")
		case a == "--requirements" && i+1 < len(args):
			i++
			f.Requirements = splitIDs(args[i])
		case strings.HasPrefix(a, "-"):
			return 0, fmt.Errorf("zumen tickets new: unknown flag %q", a)
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) != 2 {
		return 0, errors.New(usageLine("tickets", "new") + " (quote a multi-word title)")
	}
	f.Type, f.Title = pos[0], pos[1]
	st, err := state.LoadState(root)
	if err != nil {
		return 0, err
	}
	if st.Name == "" {
		return 0, fmt.Errorf("zumen: no active change — run `zumen switch <change>` first")
	}
	if st.Status == "closed" {
		return 0, fmt.Errorf("zumen: %s is closed", st.Name)
	}
	if err := resolveTicketRepo(root, st, &f); err != nil {
		return 0, err
	}
	if err := checkRequirements(root, st.Name, f.Requirements); err != nil {
		return 0, err
	}
	all, problems, err := loadTickets(root)
	if err != nil {
		return 0, err
	}
	if len(problems) > 0 {
		return 0, fmt.Errorf("zumen: can't allocate an id while a ticket is unreadable: %s", problems[0])
	}
	f.ID, err = ticket.NextID(all, f.Type)
	if err != nil {
		return 0, err
	}
	f.Change = st.Name
	f.Spec = "specs/" + st.Name + "/functional-spec.md"
	if f.Body, err = templateBody(); err != nil {
		return 0, err
	}
	t, err := ticket.New(f)
	if err != nil {
		return 0, err
	}
	path := filepath.Join(ticketDir(root, st.Name), f.ID+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, t.Bytes(), 0o644); err != nil {
		return 0, err
	}
	// Regenerated, but not recorded: the next recorded transition commits it.
	refreshOverviews(root, st.Name)
	fmt.Printf("created %s %s\n", f.ID, path)
	return 0, nil
}

// splitIDs splits a comma-separated id list, trimming spaces and dropping
// empty entries and repeats.
func splitIDs(list string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, id := range strings.Split(list, ",") {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

var requirementIDRe = regexp.MustCompile(`^[RN]\d+$`)

// checkRequirements refuses requirement ids that aren't well formed or aren't
// in the change's spec, naming every one.
func checkRequirements(root, change string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	m, err := plan.Load(root, change)
	if err != nil {
		return err
	}
	defined := map[string]bool{}
	for _, r := range m.Requirements {
		defined[r.ID] = true
	}
	var bad []string
	for _, id := range ids {
		switch {
		case !requirementIDRe.MatchString(id):
			bad = append(bad, fmt.Sprintf("%s isn't a requirement id (R<n> or N<n>)", id))
		case !m.HasSpec:
			bad = append(bad, fmt.Sprintf("%s can't be checked: %s has no spec", id, change))
		case !defined[id]:
			bad = append(bad, fmt.Sprintf("%s isn't in %s's spec", id, change))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("zumen tickets new: %s", strings.Join(bad, "; "))
	}
	return nil
}

// resolveTicketRepo fills in or checks a ticket's repo. A story or task needs one; a
// change with exactly one repo supplies it.
func resolveTicketRepo(root string, st state.State, f *ticket.Ticket) error {
	if f.Repo == "" && (f.Type == ticket.Story || f.Type == ticket.Task) {
		if len(st.Repos) != 1 {
			return fmt.Errorf("zumen: a %s needs --repo <name>", f.Type)
		}
		f.Repo = st.Repos[0]
	}
	if f.Repo == "" {
		return nil
	}
	reg, err := repos.Load(root, "zumen")
	if err != nil {
		return err
	}
	if _, ok := reg.Repos[f.Repo]; !ok {
		return fmt.Errorf("zumen: repo %q isn't registered — run `zumen repo add`", f.Repo)
	}
	return nil
}

// gateStatus is a change's status for phase, or "unset" when it has none.
func gateStatus(gates map[string]string, phase string) string {
	if s := gates[phase]; s != "" {
		return s
	}
	return "unset"
}

// checkGate checks that change has phase at want, for cmd. When it doesn't, the
// violation goes through gatePolicy: a refusal comes back as problem, and a
// violation let through as the override line to log. Both are empty when the
// gate is at want.
func checkGate(root, cmd, change, phase, want string, force bool) (override string, problem *ticket.Problem, err error) {
	gates, err := state.ChangeGates(root, change)
	if err != nil {
		return "", nil, err
	}
	status := gateStatus(gates, phase)
	if status == want {
		return "", nil, nil
	}
	violation := fmt.Errorf("zumen: %s requires %s %s for %s (it's %q)", cmd, phase, want, change, status)
	if err := gatePolicy(root, violation, force); err != nil {
		return "", &ticket.Problem{ID: change, Message: err.Error()}, nil
	}
	return "override: " + violation.Error(), nil, nil
}

func ticketsCheck(root string) (int, error) {
	all, problems, err := loadTickets(root)
	if err != nil {
		return 0, err
	}
	reg, err := repos.Load(root, "zumen")
	if err != nil {
		return 0, err
	}
	problems = append(problems, ticket.ValidateWorkspace(all, reg.Names())...)
	// A ready ticket whose breakdown was reopened is warned about, not reverted.
	changes, err := state.ReadChanges(root)
	if err != nil {
		return 0, err
	}
	var warnings []string
	for _, t := range all {
		if t.Status != ticket.StatusReady {
			continue
		}
		var gates map[string]string
		if c := changes[t.Change]; c != nil {
			gates = c.Gates
		}
		if s := gateStatus(gates, "breakdown"); s != "approved" {
			warnings = append(warnings, fmt.Sprintf("%s: ready, but breakdown for %s is %q", t.ID, t.Change, s))
		}
	}
	// Requirement, coverage and ADR findings for every change (S-0027).
	m, err := plan.LoadAll(root)
	if err != nil {
		return 0, err
	}
	for _, f := range plan.Problems(m) {
		if f.Severity == plan.Error {
			problems = append(problems, ticket.Problem{ID: f.ID, Message: f.Message})
		} else {
			warnings = append(warnings, f.ID+": "+f.Message)
		}
	}
	for _, p := range problems {
		fmt.Printf("✗ %s\n", p)
	}
	for _, w := range warnings {
		fmt.Printf("! %s\n", w)
	}
	if len(problems) > 0 {
		return 1, nil
	}
	if len(warnings) > 0 {
		fmt.Printf("%d tickets ok, %d warning(s)\n", len(all), len(warnings))
		return 0, nil
	}
	fmt.Printf("%d tickets ok\n", len(all))
	return 0, nil
}

func ticketsStatus(root string) (int, error) {
	all, problems, err := loadTickets(root)
	if err != nil {
		return 0, err
	}
	buckets := map[string][]string{}
	for _, t := range all {
		bucket := t.Status
		if t.Drifted() {
			bucket = "drifted"
		} else if t.Status == ticket.StatusPublished && !t.Frozen() {
			bucket = "unfrozen"
		}
		buckets[bucket] = append(buckets[bucket], t.ID)
	}
	for _, bucket := range []string{ticket.StatusDraft, ticket.StatusReady, ticket.StatusPublished, "unfrozen", "drifted"} {
		ids := buckets[bucket]
		sort.Strings(ids)
		fmt.Printf("%-10s %d  %s\n", bucket, len(ids), strings.Join(ids, " "))
	}
	for _, p := range problems {
		fmt.Fprintf(os.Stderr, "✗ %s\n", p)
	}
	return 0, nil
}

func ticketsOrder(root string) (int, error) {
	all, problems, err := loadTickets(root)
	if err != nil {
		return 0, err
	}
	if len(problems) > 0 {
		return 0, fmt.Errorf("zumen: unreadable ticket: %s", problems[0])
	}
	ordered, err := ticket.Order(all)
	if err != nil {
		return 0, fmt.Errorf("zumen: %v", err)
	}
	for _, t := range ordered {
		fmt.Printf("%s  %s\n", t.ID, t.Title)
	}
	return 0, nil
}

func findTicket(root, id string) (*ticket.Ticket, error) {
	all, _, err := loadTickets(root)
	if err != nil {
		return nil, err
	}
	var found *ticket.Ticket
	for _, t := range all {
		if t.ID == id {
			if found != nil {
				return nil, fmt.Errorf("zumen: more than one ticket has id %s", id)
			}
			found = t
		}
	}
	if found == nil {
		return nil, fmt.Errorf("zumen: no ticket %s", id)
	}
	return found, nil
}

// saveTicket writes a ticket, logs the lines and commits them together. It takes
// effect in the same commit, so a sync that fails partway resumes from what's
// already recorded.
func saveTicket(root string, t *ticket.Ticket, lines ...string) error {
	if err := os.WriteFile(t.Path, t.Bytes(), 0o644); err != nil {
		return err
	}
	rel, err := filepath.Rel(root, t.Path)
	if err != nil {
		return err
	}
	logged := make([]string, len(lines))
	for i, line := range lines {
		logged[i] = t.Change + ": " + line
	}
	paths := append([]string{filepath.ToSlash(rel)}, refreshOverviews(root, plan.TicketChange(t))...)
	return state.Record(root, logged, paths...)
}

// refreshOverviews regenerates the changes' READMEs under `overview: auto`
// and returns the paths it wrote, for the caller's Record. A failure is a
// warning: the command still succeeds and that README is left alone.
func refreshOverviews(root string, changes ...string) []string {
	written, err := state.RefreshOverview(root, changes...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	return written
}

// ticketsReady readies tickets once their change has breakdown approved. It's
// all-or-nothing: it checks readied copies of every target first, and writes them
// only when none has a problem.
func ticketsReady(root string, args []string) (int, error) {
	var ids []string
	var all, force bool
	for _, a := range args {
		switch {
		case a == "--all":
			all = true
		case a == "--force":
			force = true
		case strings.HasPrefix(a, "-"):
			return 0, fmt.Errorf("zumen tickets ready: unknown flag %q", a)
		default:
			ids = append(ids, a)
		}
	}
	if all == (len(ids) > 0) {
		return 0, errors.New(usageLine("tickets", "ready"))
	}
	loaded, unreadable, err := loadTickets(root)
	if err != nil {
		return 0, err
	}
	// An unreadable ticket can't be checked, so the run can't be all-or-nothing.
	problems := unreadable
	byID := map[string][]*ticket.Ticket{}
	for _, t := range loaded {
		byID[t.ID] = append(byID[t.ID], t)
	}

	var targets []*ticket.Ticket
	if all {
		st, err := state.LoadState(root)
		if err != nil {
			return 0, err
		}
		if st.Name == "" {
			return 0, fmt.Errorf("zumen: no active change — run `zumen switch <change>` first")
		}
		for _, t := range loaded {
			if t.Change == st.Name && t.Status == ticket.StatusDraft {
				targets = append(targets, t)
			}
		}
		if len(targets) == 0 && len(problems) == 0 {
			fmt.Printf("no draft tickets in %s\n", st.Name)
			return 0, nil
		}
	} else {
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			switch found := byID[id]; len(found) {
			case 0:
				problems = append(problems, ticket.Problem{ID: id, Message: "no such ticket"})
			case 1:
				targets = append(targets, found[0])
			default:
				problems = append(problems, ticket.Problem{ID: id, Message: "more than one ticket has this id"})
			}
		}
	}
	targeted := map[string]bool{}
	for _, t := range targets {
		targeted[t.ID] = true
	}

	// The breakdown gate, once per change.
	var lines []string
	checked := map[string]bool{}
	for _, t := range targets {
		if checked[t.Change] {
			continue
		}
		checked[t.Change] = true
		override, problem, err := checkGate(root, "tickets ready", t.Change, "breakdown", "approved", force)
		if err != nil {
			return 0, err
		}
		if problem != nil {
			problems = append(problems, *problem)
		}
		if override != "" {
			lines = append(lines, t.Change+": "+override)
		}
	}

	// Check a readied copy of each target.
	var copies []*ticket.Ticket
	for _, t := range targets {
		c, err := ticket.Parse(t.Bytes())
		if err != nil {
			return 0, err
		}
		c.Path = t.Path
		if err := c.MarkReady(); err != nil {
			problems = append(problems, ticket.Problem{ID: t.ID, Message: err.Error()})
			continue
		}
		problems = append(problems, ticket.ValidateFile(c)...)
		if c.Parent != "" && !targeted[c.Parent] {
			switch found := byID[c.Parent]; {
			case len(found) == 0:
				problems = append(problems, ticket.Problem{ID: t.ID, Message: fmt.Sprintf("parent %s not found", c.Parent)})
			case found[0].Status != ticket.StatusReady && found[0].Status != ticket.StatusPublished:
				problems = append(problems, ticket.Problem{ID: t.ID, Message: fmt.Sprintf("parent %s is %s; ready it first or in the same run", c.Parent, found[0].Status)})
			}
		}
		copies = append(copies, c)
	}
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Printf("✗ %s\n", p)
		}
		return 1, nil
	}

	var paths []string
	for _, c := range copies {
		if err := os.WriteFile(c.Path, c.Bytes(), 0o644); err != nil {
			return 0, err
		}
		rel, err := filepath.Rel(root, c.Path)
		if err != nil {
			return 0, err
		}
		paths = append(paths, filepath.ToSlash(rel))
		lines = append(lines, fmt.Sprintf("%s: %s readied", c.Change, c.ID))
	}
	var changes []string
	for _, c := range copies {
		changes = append(changes, plan.TicketChange(c))
	}
	paths = append(paths, refreshOverviews(root, changes...)...)
	if err := state.Record(root, lines, paths...); err != nil {
		return 0, err
	}
	for _, c := range copies {
		fmt.Printf("%s: ready\n", c.ID)
	}
	return 0, nil
}

// ticketsMarkPublished freezes a ticket with a PM tool's item id. It checks first,
// and changes nothing if any check fails: the change must be in publish (which
// --force overrides), the ticket must be validly ready or already published, and
// everything it links to must have an id in the same tool.
func ticketsMarkPublished(root string, args []string) (int, error) {
	var id, external string
	var force bool
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--external" && i+1 < len(args):
			i++
			external = args[i]
		case a == "--force":
			force = true
		case strings.HasPrefix(a, "-") || id != "":
			return 0, errors.New(usageLine("tickets", "mark-published"))
		default:
			id = a
		}
	}
	tool, itemID, ok := strings.Cut(external, "=")
	if id == "" || !ok {
		return 0, errors.New(usageLine("tickets", "mark-published"))
	}
	loaded, unreadable, err := loadTickets(root)
	if err != nil {
		return 0, err
	}
	byID := map[string][]*ticket.Ticket{}
	for _, t := range loaded {
		byID[t.ID] = append(byID[t.ID], t)
	}
	switch found := byID[id]; {
	case len(found) == 0:
		return 0, fmt.Errorf("zumen: no ticket %s", id)
	case len(found) > 1:
		return 0, fmt.Errorf("zumen: more than one ticket has id %s", id)
	}
	t := byID[id][0]

	// An unreadable ticket may be a predecessor, so it can't be verified.
	problems := unreadable
	var lines []string
	override, problem, err := checkGate(root, "mark-published", t.Change, "publish", "in-progress", force)
	if err != nil {
		return 0, err
	}
	if problem != nil {
		problems = append(problems, *problem)
	}
	if override != "" {
		lines = append(lines, override)
	}
	problems = append(problems, publishProblems(t, tool, byID)...)
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Printf("✗ %s\n", p)
		}
		return 1, nil
	}

	if err := t.MarkPublished(tool, itemID); err != nil {
		return 0, fmt.Errorf("zumen: %v", err)
	}
	lines = append(lines, fmt.Sprintf("%s published to %s (%s)", id, tool, itemID))
	if err := saveTicket(root, t, lines...); err != nil {
		return 0, err
	}
	fmt.Printf("%s: published to %s (%s)\n", id, tool, itemID)
	return 0, nil
}

// publishProblems checks a ticket's status, and that its parent, each depends_on
// ticket and its supersedes target have an id in tool. Unfrozen counts: the id is
// what a link needs. None of these can be forced.
func publishProblems(t *ticket.Ticket, tool string, byID map[string][]*ticket.Ticket) []ticket.Problem {
	var problems []ticket.Problem
	add := func(format string, args ...any) {
		problems = append(problems, ticket.Problem{ID: t.ID, Message: fmt.Sprintf(format, args...)})
	}
	switch t.Status {
	case ticket.StatusDraft:
		add("is a draft — run `zumen tickets ready %s` first", t.ID)
	case ticket.StatusReady:
		problems = append(problems, ticket.ValidateFile(t)...)
	case ticket.StatusPublished:
	default:
		add("can't publish a %s ticket", t.Status)
	}
	type ref struct{ role, id string }
	refs := []ref{{"parent", t.Parent}}
	for _, d := range t.DependsOn {
		refs = append(refs, ref{"depends_on", d})
	}
	refs = append(refs, ref{"supersedes", t.Supersedes})
	seen := map[string]bool{}
	for _, r := range refs {
		if r.id == "" || seen[r.id] {
			continue
		}
		seen[r.id] = true
		switch found := byID[r.id]; {
		case len(found) == 0:
			add("%s %s not found", r.role, r.id)
		case len(found) > 1:
			add("%s %s is ambiguous (more than one ticket has this id)", r.role, r.id)
		case found[0].External[tool] == "":
			add("%s %s has no %s id; publish it first", r.role, r.id, tool)
		}
	}
	return problems
}

func ticketsUnfreeze(root string, args []string) (int, error) {
	pos, reason, _, err := parseTransition(args)
	if err != nil || len(pos) != 1 || reason == "" {
		return 0, errors.New(usageLine("tickets", "unfreeze"))
	}
	t, err := findTicket(root, pos[0])
	if err != nil {
		return 0, err
	}
	if err := t.Unfreeze(); err != nil {
		return 0, fmt.Errorf("zumen: %v", err)
	}
	if err := saveTicket(root, t, fmt.Sprintf("%s unfrozen (%s)", pos[0], reason)); err != nil {
		return 0, err
	}
	fmt.Printf("%s: unfrozen\n", pos[0])
	return 0, nil
}
