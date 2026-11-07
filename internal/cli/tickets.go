package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/btdstudio/kiso/repos"
	"github.com/btdstudio/kiso/ticket"
	"github.com/btdstudio/zumen"
	"github.com/btdstudio/zumen/internal/state"
)

const ticketsUsage = `usage:
  zumen tickets new <type> <title> [--parent <id>] [--repo <name>] [--depends <id,id>]
  zumen tickets check
  zumen tickets status
  zumen tickets order
  zumen tickets mark-published <id> --external <tool>=<item-id>
  zumen tickets unfreeze <id> --reason <r>
`

func runTickets(root string, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, ticketsUsage)
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
	case "mark-published":
		code, err = ticketsMarkPublished(root, args[1:])
	case "unfreeze":
		code, err = ticketsUnfreeze(root, args[1:])
	default:
		fmt.Fprint(os.Stderr, ticketsUsage)
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
		case strings.HasPrefix(a, "-"):
			return 0, fmt.Errorf("zumen tickets new: unknown flag %q", a)
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) != 2 {
		return 0, fmt.Errorf("usage: zumen tickets new <type> <title> [--parent <id>] [--repo <name>] [--depends <id,id>] (quote a multi-word title)")
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
	fmt.Printf("created %s %s\n", f.ID, path)
	return 0, nil
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
	for _, p := range problems {
		fmt.Printf("✗ %s\n", p)
	}
	if len(problems) > 0 {
		return 1, nil
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

// saveTicket writes a ticket, logs the line and commits both. It takes effect in the
// same commit, so a sync that fails partway resumes from what's already recorded.
func saveTicket(root string, t *ticket.Ticket, line string) error {
	if err := os.WriteFile(t.Path, t.Bytes(), 0o644); err != nil {
		return err
	}
	rel, err := filepath.Rel(root, t.Path)
	if err != nil {
		return err
	}
	return state.Record(root, []string{t.Change + ": " + line}, filepath.ToSlash(rel))
}

func ticketsMarkPublished(root string, args []string) (int, error) {
	var id, external string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--external" && i+1 < len(args):
			i++
			external = args[i]
		case strings.HasPrefix(a, "-") || id != "":
			return 0, fmt.Errorf("usage: zumen tickets mark-published <id> --external <tool>=<item-id>")
		default:
			id = a
		}
	}
	tool, itemID, ok := strings.Cut(external, "=")
	if id == "" || !ok {
		return 0, fmt.Errorf("usage: zumen tickets mark-published <id> --external <tool>=<item-id>")
	}
	t, err := findTicket(root, id)
	if err != nil {
		return 0, err
	}
	if t.Status == ticket.StatusDraft {
		return 0, fmt.Errorf("zumen: %s is a draft — set status: ready before publishing it", id)
	}
	if err := t.MarkPublished(tool, itemID); err != nil {
		return 0, fmt.Errorf("zumen: %v", err)
	}
	if err := saveTicket(root, t, fmt.Sprintf("%s published to %s (%s)", id, tool, itemID)); err != nil {
		return 0, err
	}
	fmt.Printf("%s: published to %s (%s)\n", id, tool, itemID)
	return 0, nil
}

func ticketsUnfreeze(root string, args []string) (int, error) {
	pos, reason, _, err := parseTransition(args)
	if err != nil || len(pos) != 1 || reason == "" {
		return 0, fmt.Errorf("usage: zumen tickets unfreeze <id> --reason <r>")
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
