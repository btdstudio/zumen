package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/btdstudio/zumen/internal/idea"
)

// today is the date written into ideas and their logs. Tests replace it.
var today = func() string { return time.Now().Format("2006-01-02") }

// runIdea runs `zumen idea <verb>`.
func runIdea(root string, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usageText("idea"))
		return 1
	}
	verb, rest := args[0], args[1:]
	switch verb {
	case "new":
		return ideaNew(root, rest)
	case "list":
		return ideaList(root)
	case "show":
		return ideaShow(root, rest)
	case "park", "drop", "reopen":
		return ideaStatus(root, verb, rest)
	case "promote":
		return ideaPromote(root, rest)
	}
	fmt.Fprintf(os.Stderr, "zumen idea: unknown verb %q\n", verb)
	fmt.Fprint(os.Stderr, usageText("idea"))
	return 1
}

func ideaNew(root string, args []string) int {
	var title string
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "zumen idea new: unknown flag %q\n", a)
			return 1
		case title != "":
			fmt.Fprintf(os.Stderr, "zumen idea new: unexpected argument %q (quote a multi-word title)\n", a)
			return 1
		default:
			title = a
		}
	}
	if strings.TrimSpace(title) == "" {
		fmt.Fprintln(os.Stderr, usageLine("idea", "new"))
		return 1
	}
	i, err := idea.New(root, title, today())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("created idea %d %s (%s/)\n", i.ID, i.Title, i.Dir)
	return 0
}

func ideaList(root string) int {
	ideas, err := idea.List(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, i := range ideas {
		line := fmt.Sprintf("%d [%s] %s", i.ID, i.Status, i.Title)
		if (i.Status == idea.Parked || i.Status == idea.Dropped) && i.Reason != "" {
			line += " — reason: " + i.Reason
		}
		fmt.Println(line)
	}
	return 0
}

// parseIdeaID reads an idea id, and says what was wrong with it.
func parseIdeaID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id: %s", s)
	}
	return id, nil
}

func ideaShow(root string, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, usageLine("idea", "show"))
		return 1
	}
	id, err := parseIdeaID(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	i, err := idea.Load(root, id)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(i.Dir), "summary.md"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Print(string(data))
	return 0
}

// ideaStatus runs park, drop and reopen, which all take an id and a reason.
func ideaStatus(root, verb string, args []string) int {
	var idArg, reason string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--reason" && i+1 < len(args):
			i++
			reason = args[i]
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "zumen idea %s: unknown flag %q, or it needs a value\n", verb, a)
			return 1
		case idArg != "":
			fmt.Fprintln(os.Stderr, usageLine("idea", verb))
			return 1
		default:
			idArg = a
		}
	}
	if idArg == "" || strings.TrimSpace(reason) == "" {
		fmt.Fprintln(os.Stderr, usageLine("idea", verb))
		return 1
	}
	id, err := parseIdeaID(idArg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	do := map[string]func(string, int, string, string) (idea.Idea, error){
		"park": idea.Park, "drop": idea.Drop, "reopen": idea.Reopen,
	}[verb]
	i, err := do(root, id, reason, today())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("idea %d: %s\n", i.ID, i.Status)
	return 0
}

// ideaPromote starts a change from an idea. It checks the idea first, creates the
// change and intake through newChange, as `change new` does, and then records the
// change on the idea. If that last step fails the change is valid, so it's kept,
// and the error says how to finish by hand.
func ideaPromote(root string, args []string) int {
	var idArg, title, rev string
	var repos []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--title" && i+1 < len(args):
			i++
			title = args[i]
		case a == "--repo" && i+1 < len(args):
			i++
			repos = append(repos, args[i])
		case a == "--rev" && i+1 < len(args):
			i++
			rev = args[i]
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "zumen idea promote: unknown flag %q, or it needs a value\n", a)
			return 1
		case idArg != "":
			fmt.Fprintln(os.Stderr, usageLine("idea", "promote"))
			return 1
		default:
			idArg = a
		}
	}
	if idArg == "" || strings.TrimSpace(title) == "" {
		fmt.Fprintln(os.Stderr, usageLine("idea", "promote"))
		return 1
	}
	id, err := parseIdeaID(idArg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	i, err := idea.Load(root, id)
	if err == nil {
		err = i.CanPromote()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	date := today()
	body := fmt.Sprintf("Promoted from idea `%s/` on %s.\n\n%s", i.Dir, date, i.Body)
	st, sourceID, err := newChange(root, changeSpec{Title: title, Rev: rev, Body: body, Repos: repos, Idea: id})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printCreatedChange(st, sourceID)
	if _, err := idea.MarkPromoted(root, id, st.Name, date); err != nil {
		fmt.Fprintf(os.Stderr, "zumen: created change %s, but couldn't update idea %d: %v; add the change to its changes line by hand\n", st.Name, id, err)
		return 1
	}
	fmt.Printf("idea %d: promoted\n", id)
	return 0
}
