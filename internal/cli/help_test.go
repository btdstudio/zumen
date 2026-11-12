package cli

import (
	"bytes"
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/btdstudio/zumen"
)

// chdir moves into dir for the test; Run works on ".". Go 1.22 has no t.Chdir.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

// snapshot hashes every file under root.
func snapshot(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[p] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// helpCases are every command path, group and repo verb, each with the usage
// --help should print for it. TestHelpCommand covers help itself.
func helpCases() map[string]string {
	cases := map[string]string{}
	for _, c := range commands {
		if c.path[0] == "help" {
			continue
		}
		cases[strings.Join(c.path, " ")] = helpText(c.path)
	}
	for _, g := range []string{"source", "adr", "tickets", "change"} {
		cases[g] = helpText([]string{g})
	}
	for _, verb := range []string{"add", "list", "path", "doctor"} {
		cases["repo "+verb] = helpText([]string{"repo"})
	}
	return cases
}

func TestHelpEveryCommand(t *testing.T) {
	root := workspace(t)
	chdir(t, root)
	for name, want := range helpCases() {
		path := strings.Fields(name)
		for _, flag := range []string{"--help", "-h"} {
			for pos, args := range map[string][]string{
				"first":  append([]string{flag}, path...),
				"middle": append(slices.Clone(path), flag, "extra"),
				"last":   append(slices.Clone(path), flag),
			} {
				before := snapshot(t, root)
				code, out := captureStdout(t, func() int { return Run(args) })
				if code != 0 {
					t.Errorf("%q (%s) = %d, want 0", args, pos, code)
				}
				if out != want {
					t.Errorf("%q (%s) printed %q, want %q", args, pos, out, want)
				}
				if !strings.HasPrefix(out, "usage:") {
					t.Errorf("%q (%s) doesn't start with usage: %q", args, pos, out)
				}
				if after := snapshot(t, root); !mapsEqual(before, after) {
					t.Errorf("%q (%s) changed the workspace", args, pos)
				}
			}
		}
	}
}

func mapsEqual(a, b map[string][32]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestHelpWinsOverAFlagValue(t *testing.T) {
	root := workspace(t)
	chdir(t, root)
	before := snapshot(t, root)
	code, out := captureStdout(t, func() int { return Run([]string{"adr", "new", "--title", "--help"}) })
	if code != 0 || out != helpText([]string{"adr", "new"}) {
		t.Fatalf("adr new --title --help = %d, %q", code, out)
	}
	if !mapsEqual(before, snapshot(t, root)) {
		t.Fatal("adr new --title --help wrote files")
	}
}

func TestHelpStopsAtDoubleDash(t *testing.T) {
	if _, ok := Help([]string{"adr", "new", "--", "--help"}, &bytes.Buffer{}); ok {
		t.Fatal("--help after -- was taken as a help request")
	}
}

func TestHelpListsFlags(t *testing.T) {
	for cmd, flags := range map[string][]string{
		"adr new":    {"--title", "--deciders", "--context", "--decision", "--consequences", "--file"},
		"source new": {"--title", "--repo", "--rev", "--body"},
	} {
		var out bytes.Buffer
		if code, ok := Help(append(strings.Fields(cmd), "--help"), &out); !ok || code != 0 {
			t.Fatalf("%s --help = %d, %v", cmd, code, ok)
		}
		usage, _, _ := strings.Cut(out.String(), "\n") // the synopsis line, not the notes
		for _, f := range flags {
			if !strings.Contains(usage+" ", f+" ") {
				t.Errorf("%s --help usage line lacks %s: %q", cmd, f, usage)
			}
		}
	}
}

func TestHelpMatchesListing(t *testing.T) {
	var out bytes.Buffer
	PrintUsage(&out)
	listing := out.String()
	for _, s := range Synopses() {
		if !strings.Contains(listing, "  "+s+"\n") && !strings.Contains(listing, "  "+s+" ") {
			t.Errorf("zumen help lacks %q", s)
		}
	}
	for _, want := range []string{"zumen tickets ready", "--requirements <id,id>"} {
		if !strings.Contains(listing, want) {
			t.Errorf("zumen help lacks %q", want)
		}
	}
	// Each command's own --help shows the synopsis the listing shows.
	for _, c := range commands {
		if c.raw != "" || c.path[0] == "help" {
			continue
		}
		var help bytes.Buffer
		Help(append(slices.Clone(c.path), "--help"), &help)
		if !strings.HasPrefix(help.String(), "usage: "+c.synopsis+"\n") {
			t.Errorf("%s --help = %q, want synopsis %q", strings.Join(c.path, " "), help.String(), c.synopsis)
		}
	}
}

func TestHelpCommand(t *testing.T) {
	var viaHelp, viaFlag, top, bare bytes.Buffer
	Help([]string{"help", "adr", "new"}, &viaHelp)
	Help([]string{"adr", "new", "--help"}, &viaFlag)
	if viaHelp.String() != viaFlag.String() {
		t.Fatalf("help adr new = %q, adr new --help = %q", viaHelp.String(), viaFlag.String())
	}
	PrintUsage(&bare)
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"help", "--help"}, {"--help", "help"}} {
		top.Reset()
		if code, ok := Help(args, &top); !ok || code != 0 || top.String() != bare.String() {
			t.Fatalf("%q = %d, %v, %q, want the full listing", args, code, ok, top.String())
		}
	}
	if code, ok := Help([]string{"help", "bogus"}, &bytes.Buffer{}); !ok || code != 1 {
		t.Fatalf("help bogus = %d, %v, want 1", code, ok)
	}
	if _, ok := Help([]string{"bogus", "--help"}, &bytes.Buffer{}); ok {
		t.Fatal("bogus --help should fall through to the unknown-command error")
	}
}

func TestEveryCommandHasUsage(t *testing.T) {
	dispatched := []string{"init", "update", "check", "version", "help"} // main.run, plus Help
	for name := range handlers {
		dispatched = append(dispatched, name)
	}
	var documented []string
	for _, c := range commands {
		if !slices.Contains(documented, c.path[0]) {
			documented = append(documented, c.path[0])
		}
	}
	slices.Sort(dispatched)
	slices.Sort(documented)
	if !slices.Equal(dispatched, documented) {
		t.Fatalf("dispatched %v, documented %v", dispatched, documented)
	}
}

func TestUsageErrorsGoToStderr(t *testing.T) {
	root := workspace(t)
	chdir(t, root)
	for _, args := range [][]string{{"gate"}, {}, {"tickets"}, {"adr", "bogus"}} {
		var out string
		code, stderr := captureStderr(t, func() int {
			c, o := captureStdout(t, func() int { return Run(args) })
			out = o
			return c
		})
		if code != 1 || out != "" || !strings.Contains(strings.ToLower(stderr), "usage:") {
			t.Errorf("%q = %d, stdout %q, stderr %q; want 1, nothing, usage", args, code, out, stderr)
		}
	}
	_, stderr := captureStderr(t, func() int { return Run([]string{"gate"}) })
	if !strings.Contains(stderr, usageLine("gate")) {
		t.Errorf("gate usage = %q, want the table's %q", stderr, usageLine("gate"))
	}
}

// commandRows is the backticked first column of each `zumen` row in a
// Markdown command table, with `\|` unescaped.
func commandRows(t *testing.T, name string, data []byte) []string {
	t.Helper()
	var rows []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "| `zumen ") {
			continue
		}
		cell := strings.TrimPrefix(line, "| `")
		end := strings.Index(cell, "` |")
		if end < 0 {
			t.Fatalf("%s: malformed command row: %q", name, line)
		}
		rows = append(rows, strings.ReplaceAll(cell[:end], `\|`, "|"))
	}
	return rows
}

// TestCommandTablesMatchUsage pins the scaffold's AGENTS.md and the repo
// README command tables to the usage table, so they can't drift.
func TestCommandTablesMatchUsage(t *testing.T) {
	scaffold, err := fs.ReadFile(zumen.ScaffoldFS, "scaffold/AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := Synopses()
	for name, data := range map[string][]byte{"scaffold/AGENTS.md": scaffold, "README.md": readme} {
		if rows := commandRows(t, name, data); !slices.Equal(rows, want) {
			t.Errorf("%s command table:\n%s\nwant (zumen help):\n%s",
				name, strings.Join(rows, "\n"), strings.Join(want, "\n"))
		}
	}
}

// TestGroupHelpListsEveryVerb (AC8) checks each group's --help against verbs
// written out here, not against the table it's printed from.
func TestGroupHelpListsEveryVerb(t *testing.T) {
	for group, verbs := range map[string][]string{
		"source":  {"new", "list", "show"},
		"adr":     {"new", "list", "show"},
		"change":  {"new", "close"},
		"tickets": {"new", "check", "status", "order", "ready", "mark-published", "unfreeze"},
		"repo":    {"add", "list", "path", "doctor"},
	} {
		var out bytes.Buffer
		if code, ok := Help([]string{group, "--help"}, &out); !ok || code != 0 {
			t.Fatalf("%s --help = %d, %v", group, code, ok)
		}
		for _, v := range verbs {
			if !strings.Contains(out.String(), "  zumen "+group+" "+v) {
				t.Errorf("%s --help lacks %s: %q", group, v, out.String())
			}
		}
	}
}
