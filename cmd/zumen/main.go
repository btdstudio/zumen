// Command zumen is the planning ICM tool: scaffold a folder-driven planning
// workspace (`init`), refresh its managed files (`update`), validate the
// embedded scaffold (`check`), and a small set of workspace helpers.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/btdstudio/kiso/version"
	"github.com/btdstudio/zumen/internal/cli"
	"github.com/btdstudio/zumen/internal/icm"
	"github.com/btdstudio/zumen/internal/settings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if code, ok := cli.Help(args, os.Stdout); ok {
		return code
	}
	if len(args) == 0 {
		cli.PrintUsage(os.Stderr)
		return 1
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("zumen version %s\n", version.String())
		return 0
	case "init":
		return runInit(args[1:])
	case "update":
		return runUpdate()
	case "check":
		return runCheck()
	default:
		return cli.Run(args)
	}
}

func runInit(args []string) int {
	dir := "."
	noGit := false
	force := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--no-git":
			noGit = true
		case a == "--force":
			force = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "zumen init: unknown flag %s\n", a)
			return 1
		default:
			dir = a
		}
	}
	vcs, err := settings.InitVCS(noGit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	rep, err := icm.Scaffold(dir, icm.Options{NoGit: noGit, Force: force}, vcs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, rel := range rep.Written {
		fmt.Println("wrote", rel)
	}
	if rep.GitInit {
		fmt.Println("git init")
	}
	fmt.Printf("scaffolded planning ICM workspace in %s\n", dir)
	return 0
}

func runUpdate() int {
	rep, err := icm.Update(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, rel := range rep.Updated {
		fmt.Println("updated", rel)
	}
	for _, rel := range rep.Skipped {
		fmt.Println("skipped (edited)", rel)
	}
	fmt.Printf("update complete (%d updated, %d skipped, %d unchanged)\n",
		len(rep.Updated), len(rep.Skipped), len(rep.Unchanged))
	return 0
}

func runCheck() int {
	problems := icm.Check()
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p)
	}
	if len(problems) > 0 {
		return 1
	}
	fmt.Println("scaffold is clean")
	return 0
}
