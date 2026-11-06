// Package cli implements the zumen workspace helpers: status, settings, switch,
// source, adr, help.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/btdstudio/zumen/internal/adr"
	"github.com/btdstudio/zumen/internal/export"
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
	case "export":
		return runExport(root, args[1:])
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
  zumen init [dir] [--no-git] [--force]   scaffold the planning ICM structure
  zumen update                             refresh managed files
  zumen status                             show the active change
  zumen settings                           show effective settings
  zumen switch <change>                    switch the active change
  zumen source new|list|show [args]        manage 00_intake/
  zumen adr new|list|show [args]           manage 03_design/adr/
  zumen change new <title> [--repo <p>]    create a change + intake + switch
  zumen gate <phase> <status> [--force]    advance the active change's gate
  zumen export [<change>] --to <tomo> [--force]
                                           copy an approved spec into a tomo workspace
  zumen check                              validate the embedded scaffold
  zumen version                            print the version
  zumen help                               this help
`)
}

func runStatus(root string) int {
	st, err := state.LoadState(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if st.Change == "" {
		fmt.Println("no active change")
		return 0
	}
	fmt.Printf("change:   %s\n", st.Change)
	fmt.Printf("mode:     %s\n", st.Mode)
	fmt.Printf("phase:    %s\n", st.Phase)
	fmt.Printf("status:   %s\n", st.Status)
	fmt.Printf("gates:    intake=%s explore=%s requirements=%s design=%s plan=%s\n",
		st.Gates.Intake, st.Gates.Explore, st.Gates.Requirements, st.Gates.Design, st.Gates.Plan)
	warnModeMismatch(root, st)
	return 0
}

// warnModeMismatch warns when a change's intake is a repo but its mode is not
// "existing" (which would silently skip the explore phase).
func warnModeMismatch(root string, st state.State) {
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
		if s.Source == source.SOURCE_REPO && st.Mode != "existing" {
			fmt.Fprintf(os.Stderr, "warning: source #%d is a repo (%s) but mode is %q — the explore phase will be skipped\n", id, s.Title, st.Mode)
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
	return 0
}

func runSwitch(root string, args []string) int {
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
			fmt.Printf("%s  [%s] %s\n", c.Name, c.Mode, c.Phase)
		}
		return 0
	}
	st, err := state.SwitchTo(root, args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("switched to %s\n", st.Change)
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
	if len(args) == 0 || args[0] != "new" {
		fmt.Fprintln(os.Stderr, "usage: zumen change new <title> [--repo <path>] [--rev <r>] [--body <b>]")
		return 1
	}
	args = args[1:]
	var title, repo, rev, body string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--repo" && i+1 < len(args):
			i++
			repo = args[i]
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
		fmt.Fprintln(os.Stderr, "usage: zumen change new <title> [--repo <path>] [--rev <r>] [--body <b>]")
		return 1
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
	mode := "greenfield"
	if repo != "" {
		src = source.SOURCE_REPO
		mode = "existing"
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
	st, err := state.SeedChange(root, name, mode, []string{rel})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("created change %s [%s] (source #%d)\n", st.Change, st.Mode, created.ID)
	return 0
}

func runGate(root string, args []string) int {
	force := false
	var pos []string
	for _, a := range args {
		if a == "--force" {
			force = true
		} else {
			pos = append(pos, a)
		}
	}
	if len(pos) != 2 {
		fmt.Fprintln(os.Stderr, "usage: zumen gate <phase> <status> [--force]")
		return 1
	}
	phase, status := pos[0], pos[1]
	st, err := state.LoadState(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var violation error
	if st.Change != "" {
		probe := st
		if err := state.SetGate(&probe, phase, status); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if violation = state.CheckGate(st, phase, status); violation != nil {
			if err := gatePolicy(root, violation, force); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
	}
	st, err = state.AdvanceGate(root, phase, status)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if violation != nil {
		if err := state.AppendLog(root, st.Change+": override: "+violation.Error()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	fmt.Printf("%s: %s = %s\n", st.Change, st.Phase, st.Status)
	return 0
}

func runExport(root string, args []string) int {
	var change, target, specsDir string
	force := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--to" && i+1 < len(args):
			i++
			target = args[i]
		case a == "--specs-dir" && i+1 < len(args):
			i++
			specsDir = args[i]
		case a == "--force":
			force = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "zumen export: unknown flag %q\n", a)
			return 1
		default:
			change = a
		}
	}
	if target == "" {
		fmt.Fprintln(os.Stderr, "usage: zumen export [<change>] --to <tomo-workspace> [--specs-dir <dir>] [--force]")
		return 1
	}
	if change == "" {
		st, err := state.LoadState(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if st.Change == "" {
			fmt.Fprintln(os.Stderr, "zumen export: no active change — pass <change> or run `zumen switch <change>`")
			return 1
		}
		change = st.Change
	}
	changes, err := state.ReadChanges(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var violation error
	if rec := changes[change]; rec == nil || rec.Gates.Plan != "approved" {
		violation = fmt.Errorf("zumen: the plan for %s is not approved", change)
		if err := gatePolicy(root, violation, force); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	rep, err := export.Export(root, change, target, specsDir, force)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if violation != nil {
		if err := state.AppendLog(root, "override: export: "+violation.Error()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	for _, rel := range rep.Copied {
		fmt.Printf("exported %s/%s\n", rep.Change, rel)
	}
	fmt.Printf("exported %s -> %s\n", rep.Change, rep.Dest)
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
