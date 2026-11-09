// Package cli implements the zumen workspace helpers: status, settings, switch,
// source, adr, help.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/btdstudio/kiso/repos"
	kiso "github.com/btdstudio/kiso/state"
	"github.com/btdstudio/zumen/internal/adr"
	"github.com/btdstudio/zumen/internal/settings"
	"github.com/btdstudio/zumen/internal/source"
	"github.com/btdstudio/zumen/internal/state"
)

// Run dispatches a helper subcommand and returns a process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		PrintUsage()
		return 1
	}
	root := "."
	switch args[0] {
	case "help", "-h", "--help":
		PrintUsage()
		return 0
	case "status":
		return runStatus(root)
	case "settings":
		return runSettings(root)
	case "switch":
		return runSwitch(root, args[1:])
	case "source":
		return runSource(root, args[1:])
	case "adr":
		return runAdr(root, args[1:])
	case "change":
		return runChange(root, args[1:])
	case "gate":
		return runGate(root, args[1:])
	case "reopen":
		return runReopen(root, args[1:])
	case "tickets":
		return runTickets(root, args[1:])
	case "repo":
		return repos.Run(root, "zumen", args[1:], os.Stdout, os.Stderr)
	case "env":
		return runEnv(root)
	default:
		fmt.Fprintf(os.Stderr, "zumen: unknown command %q\n", args[0])
		PrintUsage()
		return 1
	}
}

// PrintUsage prints the zumen usage summary.
func PrintUsage() {
	fmt.Print(`zumen — folder-driven planning workspace

Usage:
  zumen init [dir] [--no-git] [--force]   scaffold the planning ICM structure (--no-git: local, vcs: none)
  zumen update                             refresh managed files
  zumen status                             show the active change
  zumen settings                           show effective settings
  zumen switch [--all] [<change>]          list changes, or switch the active one
  zumen source new|list|show [args]        manage 00_intake/
  zumen adr new|list|show [args]           manage 04_architecture/adr/
  zumen change new <title> [--repo <name>]...
                                           create a change + intake + switch
  zumen change close <change> --reason <r> close a change
  zumen gate <phase> <status> [--reason <r>] [--force]
                                           advance the active change's gate
  zumen reopen <phase> --reason <r>        move back to a phase, clearing later approvals
  zumen tickets new <type> <title> [--parent <id>] [--repo <name>] [--depends <id,id>]
                                           create a ticket in the active change
  zumen tickets check|status|order         validate, list and order tickets
  zumen tickets mark-published <id> --external <tool>=<item-id>
                                           record a PM tool's id and freeze the ticket
  zumen tickets unfreeze <id> --reason <r> clear a ticket's freeze
` + repos.Usage("zumen") + `  zumen env                                print harness flags for the active change's repos
  zumen check                              validate the embedded scaffold
  zumen version                            print the version
  zumen help                               this help
`)
}

func runStatus(root string) int {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	st, err := state.LoadState(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if st.Name == "" {
		fmt.Println("no active change")
		fmt.Printf("vcs:      %s\n", loaded.VCS)
		return 0
	}
	fmt.Printf("change:   %s\n", st.Name)
	fmt.Printf("phase:    %s\n", st.Phase)
	fmt.Printf("status:   %s\n", st.Status)
	var gates []string
	for _, p := range state.Machine.Phases {
		gates = append(gates, p.Name+"="+st.Gates[p.Name])
	}
	fmt.Printf("gates:    %s\n", strings.Join(gates, " "))
	fmt.Printf("vcs:      %s\n", loaded.VCS)
	warnSkippedExplore(root, st)
	return 0
}

// warnSkippedExplore warns when a change's intake is a repo but its explore phase
// is skipped.
func warnSkippedExplore(root string, st state.State) {
	for _, src := range st.Sources {
		dir := filepath.Dir(src)
		base := filepath.Base(src)
		id, err := strconv.Atoi(strings.TrimSuffix(base, filepath.Ext(base)))
		if err != nil {
			continue
		}
		s, err := source.ReadSource(root, dir, id)
		if err != nil {
			continue
		}
		if s.Source == source.SOURCE_REPO && st.Gates["explore"] == kiso.Skipped {
			fmt.Fprintf(os.Stderr, "warning: source #%d is a repo (%s) but the explore phase is skipped\n", id, s.Title)
		}
	}
}

func runSettings(root string) int {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	e := loaded.Effective
	fmt.Printf("baseSource:  %s\n", e.BaseSource)
	fmt.Printf("gates:       %s\n", e.Gates)
	fmt.Printf("sourceDir:   %s\n", e.SourceDir)
	fmt.Printf("artifactDir: %s\n", e.ArtifactDir)
	fmt.Printf("adrDir:      %s\n", e.AdrDir)
	fmt.Printf("syncFile:    %s\n", e.SyncFile)
	return 0
}

func runSwitch(root string, args []string) int {
	all := len(args) > 0 && args[0] == "--all"
	if all {
		args = args[1:]
	}
	if len(args) == 0 {
		changes, err := state.KnownChanges(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(changes) == 0 {
			fmt.Println("no changes yet — create a specs/<n>-<slug>/ folder first")
			return 0
		}
		for _, c := range changes {
			if c.Status == kiso.Closed && !all {
				continue
			}
			fmt.Printf("%s  %s %s\n", c.Name, c.Phase, c.Status)
		}
		return 0
	}
	st, err := state.SwitchTo(root, args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("switched to %s\n", st.Name)
	return 0
}

// gatePolicy decides whether a gate violation stops the command: "block" refuses
// it unless forced; "prompt", "auto" and a forced run warn and proceed.
func gatePolicy(root string, violation error, force bool) error {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		return err
	}
	if loaded.Effective.Gates == "block" && !force {
		return fmt.Errorf("%v (pass --force to override)", violation)
	}
	fmt.Fprintf(os.Stderr, "warning: %v\n", violation)
	return nil
}

func runChange(root string, args []string) int {
	if len(args) > 0 && args[0] == "close" {
		return runChangeClose(root, args[1:])
	}
	if len(args) == 0 || args[0] != "new" {
		fmt.Fprintln(os.Stderr, "usage: zumen change new <title> [--repo <name|path>]... [--rev <r>] [--body <b>]\n       zumen change close <change> --reason <r>")
		return 1
	}
	args = args[1:]
	var title, rev, body string
	var repoArgs []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--repo" && i+1 < len(args):
			i++
			repoArgs = append(repoArgs, args[i])
		case a == "--rev" && i+1 < len(args):
			i++
			rev = args[i]
		case a == "--body" && i+1 < len(args):
			i++
			body = args[i]
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "zumen change new: unknown flag %q\n", a)
			return 1
		case title != "":
			fmt.Fprintf(os.Stderr, "zumen change new: unexpected argument %q (quote a multi-word title)\n", a)
			return 1
		default:
			title = a
		}
	}
	if title == "" {
		fmt.Fprintln(os.Stderr, "usage: zumen change new <title> [--repo <name|path>]... [--rev <r>] [--body <b>]")
		return 1
	}
	registered, err := changeRepos(root, repoArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	repo := ""
	if len(repoArgs) > 0 {
		repo = repoArgs[0]
	}
	// Fail on unreadable state before writing the intake and spec folder.
	if _, err := state.LoadState(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	src := source.SOURCE_ISSUE
	if repo != "" {
		src = source.SOURCE_REPO
	}
	created, err := source.WriteNewSource(root, loaded.Effective.SourceDir, source.NewSourceOptions{
		Title: title, Source: src, Repo: repo, Rev: rev, Body: body,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	id := state.NextChangeId(root)
	slug := state.Slugify(title)
	if slug == "" {
		slug = "change"
	}
	name := fmt.Sprintf("%d-%s", id, slug)
	rel := filepath.ToSlash(filepath.Join(loaded.Effective.SourceDir, fmt.Sprintf("%d.md", created.ID)))
	st, err := state.SeedChange(root, name, repo != "", []string{rel}, registered...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("created change %s (source #%d)\n", st.Name, created.ID)
	for _, phase := range []string{"explore", "synthesis"} {
		if st.Gates[phase] == kiso.Skipped {
			fmt.Printf("%s skipped\n", phase)
		}
	}
	return 0
}

// changeRepos returns which --repo values are registered names. A single
// unregistered value is still accepted as a path to the codebase; several repos
// have to be registered, because a change can only track logical names.
func changeRepos(root string, args []string) ([]string, error) {
	reg, err := repos.Load(root, "zumen")
	if err != nil {
		return nil, err
	}
	var registered []string
	for _, a := range args {
		if _, ok := reg.Repos[a]; ok {
			registered = append(registered, a)
		} else if len(args) > 1 {
			return nil, fmt.Errorf("zumen: %q isn't registered — run `zumen repo add` for each repo in a multi-repo change", a)
		}
	}
	return registered, nil
}

func runEnv(root string) int {
	st, err := state.LoadState(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if st.Name == "" {
		fmt.Fprintln(os.Stderr, "zumen: no active change — run `zumen switch <change>` first")
		return 1
	}
	return repos.RunEnv(root, "zumen", st.Repos, os.Stdout, os.Stderr)
}

func runChangeClose(root string, args []string) int {
	pos, reason, _, err := parseTransition(args)
	if err != nil || len(pos) != 1 {
		fmt.Fprintln(os.Stderr, "usage: zumen change close <change> --reason <r>")
		return 1
	}
	st, err := state.CloseChange(root, pos[0], reason)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("%s: closed\n", st.Name)
	return 0
}

// parseTransition splits positional arguments from --reason and --force.
func parseTransition(args []string) (pos []string, reason string, force bool, err error) {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--force":
			force = true
		case a == "--reason" && i+1 < len(args):
			i++
			reason = args[i]
		case strings.HasPrefix(a, "-"):
			return nil, "", false, fmt.Errorf("unknown flag %q", a)
		default:
			pos = append(pos, a)
		}
	}
	return pos, reason, force, nil
}

func runGate(root string, args []string) int {
	usage := "usage: zumen gate <phase> <status> [--reason <r>] [--force]"
	pos, reason, force, err := parseTransition(args)
	if err != nil || len(pos) != 2 {
		fmt.Fprintln(os.Stderr, usage)
		return 1
	}
	phase, status := pos[0], pos[1]
	st, err := state.LoadState(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var notes []string
	if st.Name != "" {
		probe := st
		if err := state.SetGate(&probe, phase, status, reason); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if violation := state.CheckGate(st, phase, status); violation != nil {
			if err := gatePolicy(root, violation, force); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			notes = append(notes, "override: "+violation.Error())
		}
	}
	st, err = state.AdvanceGate(root, phase, status, reason, notes...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("%s: %s = %s\n", st.Name, st.Phase, st.Status)
	return 0
}

func runReopen(root string, args []string) int {
	pos, reason, _, err := parseTransition(args)
	if err != nil || len(pos) != 1 {
		fmt.Fprintln(os.Stderr, "usage: zumen reopen <phase> --reason <r>")
		return 1
	}
	st, err := state.Reopen(root, pos[0], reason)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("%s: %s = %s\n", st.Name, st.Phase, st.Status)
	return 0
}

func runSource(root string, args []string) int {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir := loaded.Effective.SourceDir
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "list":
		for _, s := range source.ListSources(root, dir) {
			label := s.Source
			if s.Source == source.SOURCE_REPO {
				label = "repo:" + s.Repo
			}
			fmt.Printf("#%d [%s] %s\n", s.ID, label, s.Title)
		}
		return 0
	case "show":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: zumen source show <id>")
			return 1
		}
		id, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid id: %s\n", args[1])
			return 1
		}
		s, err := source.ReadSource(root, dir, id)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		extra := ""
		if s.Source == source.SOURCE_REPO {
			extra = fmt.Sprintf("\nrepo:     %s\nrev:      %s", s.Repo, s.Rev)
		}
		fmt.Printf("#%d [%s] %s%s\n%s\n", s.ID, s.Source, s.Title, extra, s.Body)
		return 0
	case "new":
		var title, repo, rev, body string
		for i := 1; i < len(args); i++ {
			switch a := args[i]; {
			case a == "--title" && i+1 < len(args):
				i++
				title = args[i]
			case a == "--repo" && i+1 < len(args):
				i++
				repo = args[i]
			case a == "--rev" && i+1 < len(args):
				i++
				rev = args[i]
			case a == "--body" && i+1 < len(args):
				i++
				body = args[i]
			default:
				fmt.Fprintf(os.Stderr, "zumen source new: unexpected argument %q (quote multi-word values)\n", a)
				return 1
			}
		}
		if title == "" {
			fmt.Fprintln(os.Stderr, "usage: zumen source new --title <t> [--repo <path>] [--rev <r>] [--body <b>]")
			return 1
		}
		src := source.SOURCE_ISSUE
		if repo != "" {
			src = source.SOURCE_REPO
		}
		s, err := source.WriteNewSource(root, dir, source.NewSourceOptions{Title: title, Source: src, Repo: repo, Rev: rev, Body: body})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("created #%d [%s] %s\n", s.ID, s.Source, s.Title)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "zumen source: unknown verb %q\n", args[0])
		return 1
	}
}

func runAdr(root string, args []string) int {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir := loaded.Effective.AdrDir
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "list":
		for _, r := range adr.List(root, dir) {
			fmt.Printf("%04d [%s] %s\n", r.Number, r.Status, r.Title)
		}
		return 0
	case "show":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: zumen adr show <number>")
			return 1
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid number: %s\n", args[1])
			return 1
		}
		r, err := adr.Show(root, dir, n)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Print(r.Raw)
		return 0
	case "new":
		var title, deciders, context, decision, consequences, file string
		for i := 1; i < len(args); i++ {
			switch a := args[i]; {
			case a == "--title" && i+1 < len(args):
				i++
				title = args[i]
			case a == "--deciders" && i+1 < len(args):
				i++
				deciders = args[i]
			case a == "--context" && i+1 < len(args):
				i++
				context = args[i]
			case a == "--decision" && i+1 < len(args):
				i++
				decision = args[i]
			case a == "--consequences" && i+1 < len(args):
				i++
				consequences = args[i]
			case a == "--file" && i+1 < len(args):
				i++
				file = args[i]
			default:
				fmt.Fprintf(os.Stderr, "zumen adr new: unexpected argument %q (quote multi-word values)\n", a)
				return 1
			}
		}
		if title == "" {
			fmt.Fprintln(os.Stderr, "usage: zumen adr new --title <t> [--deciders <d>] [--context <c>] [--decision <d>] [--consequences <c>] [--file <path>]")
			return 1
		}
		opts := adr.Options{Title: title, Deciders: deciders, Context: context, Decision: decision, Consequences: consequences}
		if file != "" {
			data, err := os.ReadFile(file)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			opts.Body = string(data)
		}
		r, err := adr.Create(root, dir, opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("created %04d %s\n", r.Number, r.Title)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "zumen adr: unknown verb %q\n", args[0])
		return 1
	}
}
