package cli

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/btdstudio/kiso/repos"
)

// command is one zumen command's usage. Its synopsis is the one source of the
// command's flags: --help, `zumen help`, the usage errors and the scaffold's
// AGENTS.md command table all come from it.
type command struct {
	path     []string // {"adr", "new"}; a shorter shared prefix is a group
	synopsis string   // every flag the command's parser accepts
	summary  string
	notes    string // extra --help lines, such as what a flag means
	raw      string // replaces synopsis and summary in listings (repo, from kiso)
}

var commands = []command{
	{path: []string{"init"}, synopsis: "zumen init [dir] [--no-git] [--force]", summary: "scaffold the planning ICM structure",
		notes: "  --no-git   a local workspace (vcs: none)\n  --force    overwrite files that already exist\n"},
	{path: []string{"update"}, synopsis: "zumen update", summary: "refresh managed files"},
	{path: []string{"status"}, synopsis: "zumen status", summary: "show the active change"},
	{path: []string{"settings"}, synopsis: "zumen settings", summary: "show effective settings"},
	{path: []string{"switch"}, synopsis: "zumen switch [--all] [<change>]", summary: "list changes, or switch the active one",
		notes: "  --all      include closed changes in the list\n"},
	{path: []string{"source", "new"}, synopsis: "zumen source new --title <t> [--repo <path>] [--rev <r>] [--body <b>]", summary: "add an intake source to 00_intake/"},
	{path: []string{"source", "list"}, synopsis: "zumen source list", summary: "list intake sources (the default verb)"},
	{path: []string{"source", "show"}, synopsis: "zumen source show <id>", summary: "show an intake source"},
	{path: []string{"adr", "new"}, synopsis: "zumen adr new --title <t> [--deciders <d>] [--context <c>] [--decision <d>] [--consequences <c>] [--file <path>]", summary: "create an ADR in 04_architecture/adr/",
		notes: "  --file     read the ADR body from a file\n"},
	{path: []string{"adr", "list"}, synopsis: "zumen adr list", summary: "list ADRs (the default verb)"},
	{path: []string{"adr", "show"}, synopsis: "zumen adr show <number>", summary: "show an ADR"},
	{path: []string{"change", "new"}, synopsis: "zumen change new <title> [--repo <name|path>]... [--rev <r>] [--body <b>]", summary: "create a change + intake + switch"},
	{path: []string{"change", "close"}, synopsis: "zumen change close <change> --reason <r>", summary: "close a change"},
	{path: []string{"gate"}, synopsis: "zumen gate <phase> <status> [--reason <r>] [--force]", summary: "advance the active change's gate"},
	{path: []string{"reopen"}, synopsis: "zumen reopen <phase> --reason <r>", summary: "move back to a phase, clearing later approvals"},
	{path: []string{"tickets", "new"}, synopsis: "zumen tickets new <type> <title> [--parent <id>] [--repo <name>] [--depends <id,id>] [--requirements <id,id>]", summary: "create a ticket in the active change"},
	{path: []string{"tickets", "check"}, synopsis: "zumen tickets check", summary: "validate every ticket"},
	{path: []string{"tickets", "status"}, synopsis: "zumen tickets status", summary: "list tickets by draft, ready, published and drifted"},
	{path: []string{"tickets", "order"}, synopsis: "zumen tickets order", summary: "list tickets in publish order"},
	{path: []string{"tickets", "ready"}, synopsis: "zumen tickets ready <id>... | --all [--force]", summary: "ready draft tickets once breakdown is approved"},
	{path: []string{"tickets", "mark-published"}, synopsis: "zumen tickets mark-published <id> --external <tool>=<item-id> [--force]", summary: "record a PM tool's id and freeze the ticket",
		notes: "  --force    allow it outside the publish phase\n"},
	{path: []string{"tickets", "unfreeze"}, synopsis: "zumen tickets unfreeze <id> --reason <r>", summary: "clear a ticket's freeze"},
	{path: []string{"overview"}, synopsis: "zumen overview [<change>] [--check] [--stdout]", summary: "write or check the change's README overview"},
	{path: []string{"references", "check"}, synopsis: "zumen references check", summary: "warn about unindexed references and bad index entries"},
	{path: []string{"repo"}, raw: repos.Usage("zumen")},
	{path: []string{"env"}, synopsis: "zumen env", summary: "print harness flags for the active change's repos"},
	{path: []string{"check"}, synopsis: "zumen check", summary: "validate the embedded scaffold"},
	{path: []string{"version"}, synopsis: "zumen version", summary: "print the version"},
	{path: []string{"help"}, synopsis: "zumen help [<command> [<verb>]]", summary: "this help, or one command's"},
}

// summaryColumn is where listings start a command's summary.
const summaryColumn = 43

// listing is c's lines in the two-column `zumen help` layout.
func (c command) listing() string {
	if c.raw != "" {
		return c.raw
	}
	line := "  " + c.synopsis
	if len(line) < summaryColumn {
		return line + strings.Repeat(" ", summaryColumn-len(line)) + c.summary + "\n"
	}
	return line + "\n" + strings.Repeat(" ", summaryColumn) + c.summary + "\n"
}

// synopses is c's synopsis lines; repo has one per kiso usage line.
func (c command) synopses() []string {
	if c.raw == "" {
		return []string{c.synopsis}
	}
	var out []string
	for _, line := range strings.Split(c.raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "zumen ") {
			continue
		}
		if i := strings.Index(line, "  "); i >= 0 {
			line = line[:i]
		}
		out = append(out, line)
	}
	return out
}

// Synopses lists every command's synopsis in `zumen help` order.
func Synopses() []string {
	var out []string
	for _, c := range commands {
		out = append(out, c.synopses()...)
	}
	return out
}

// PrintUsage prints the zumen usage summary.
func PrintUsage(w io.Writer) {
	fmt.Fprint(w, "zumen — folder-driven planning workspace\n\nUsage:\n")
	for _, c := range commands {
		fmt.Fprint(w, c.listing())
	}
}

// group returns the commands under prefix, in table order.
func group(prefix []string) []command {
	var out []command
	for _, c := range commands {
		if len(c.path) >= len(prefix) && slices.Equal(c.path[:len(prefix)], prefix) {
			out = append(out, c)
		}
	}
	return out
}

// usageText is the usage for a command or group path: "usage: <synopsis>" for
// one command, or "usage:" and each command's listing for a group.
func usageText(path ...string) string {
	cmds := group(path)
	if len(cmds) == 1 && slices.Equal(cmds[0].path, path) && cmds[0].raw == "" {
		return "usage: " + cmds[0].synopsis + "\n"
	}
	var b strings.Builder
	b.WriteString("usage:\n")
	for _, c := range cmds {
		b.WriteString(c.listing())
	}
	return b.String()
}

// usageLine is usageText without its trailing newline, for errors.
func usageLine(path ...string) string {
	return strings.TrimSuffix(usageText(path...), "\n")
}

// helpText is what --help prints for path: its usage, and for one command its
// summary and notes too.
func helpText(path []string) string {
	text := usageText(path...)
	cmds := group(path)
	if len(cmds) == 1 && cmds[0].raw == "" && slices.Equal(cmds[0].path, path) {
		text += "\n" + cmds[0].summary + "\n" + cmds[0].notes
	}
	return text
}

// Help handles a help request: --help or -h anywhere before a "--", or the
// help command. It prints the usage of the command named by the leading words
// to stdout and returns 0, and ok is false when args aren't a help request or
// name no known command. It reads and writes nothing else, so a command asked
// for help never runs.
func Help(args []string, stdout io.Writer) (code int, ok bool) {
	help := false
	var words []string
	for i, a := range args {
		if a == "--" {
			break
		}
		if a == "--help" || a == "-h" || (i == 0 && a == "help") {
			help = true
			continue
		}
		words = append(words, a)
	}
	if !help {
		return 0, false
	}
	var path []string
	for _, w := range words {
		if strings.HasPrefix(w, "-") || len(group(append(slices.Clone(path), w))) == 0 {
			break
		}
		path = append(path, w)
	}
	if len(path) == 0 || slices.Equal(path, []string{"help"}) {
		if len(path) == 0 && len(words) > 0 && !strings.HasPrefix(words[0], "-") {
			if args[0] == words[0] {
				return 0, false // `zumen bogus --help`: dispatch reports it
			}
			fmt.Fprintf(os.Stderr, "zumen: unknown command %q\n", words[0])
			PrintUsage(os.Stderr)
			return 1, true
		}
		PrintUsage(stdout)
		return 0, true
	}
	fmt.Fprint(stdout, helpText(path))
	return 0, true
}
