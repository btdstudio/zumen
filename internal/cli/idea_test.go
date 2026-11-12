package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/zumen/internal/state"
)

// ideaRun runs `zumen idea <args>` and returns the exit code, stdout and stderr.
func ideaRun(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var out string
	code, errOut := captureStderr(t, func() int {
		c, o := captureStdout(t, func() int { return runIdea(root, args) })
		out = o
		return c
	})
	return code, out, errOut
}

func setToday(t *testing.T, date string) {
	t.Helper()
	orig := today
	today = func() string { return date }
	t.Cleanup(func() { today = orig })
}

func ideaFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func mustIdea(t *testing.T, root string, args ...string) {
	t.Helper()
	if code, out, errOut := ideaRun(t, root, args...); code != 0 {
		t.Fatalf("zumen idea %q = %d, %q, %q", args, code, out, errOut)
	}
}

// AC2: new creates the folder, and refuses an unquoted multi-word title.
func TestIdeaNew(t *testing.T) {
	setToday(t, "2026-11-12")
	root := workspace(t)
	for _, want := range []string{"created idea 1 Faster imports (ideas/1-faster-imports/)", "created idea 2 Faster imports (ideas/2-faster-imports/)"} {
		code, out, _ := ideaRun(t, root, "new", "Faster imports")
		if code != 0 || strings.TrimSpace(out) != want {
			t.Fatalf("new = %d, %q; want %q", code, out, want)
		}
	}
	for _, dir := range []string{"ideas/1-faster-imports", "ideas/2-faster-imports"} {
		if s := ideaFile(t, root, dir+"/summary.md"); !strings.Contains(s, "status: open") {
			t.Errorf("%s/summary.md = %q", dir, s)
		}
		ideaFile(t, root, dir+"/log.md")
	}
	for _, args := range [][]string{{"new"}, {"new", "  "}, {"new", "Faster", "imports"}, {"new", "--title", "X"}} {
		code, out, errOut := ideaRun(t, root, args...)
		if code != 1 || out != "" || errOut == "" {
			t.Errorf("%q = %d, %q, %q; want exit 1, an error and no stdout", args, code, out, errOut)
		}
	}
	if _, _, errOut := ideaRun(t, root, "new", "Faster", "imports"); !strings.Contains(errOut, "quote a multi-word title") {
		t.Errorf("the unquoted title error = %q", errOut)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "ideas")); len(entries) != 2 {
		t.Errorf("a refused new wrote %v", entries)
	}
}

// AC6: list shows every idea, and a reason for parked and dropped ones; show
// prints the summary.
func TestIdeaListAndShow(t *testing.T) {
	setToday(t, "2026-11-12")
	root := workspace(t)
	if code, out, _ := ideaRun(t, root, "list"); code != 0 || out != "" {
		t.Fatalf("an empty list = %d, %q", code, out)
	}
	for _, title := range []string{"Open one", "Parked one", "Dropped one", "Promoted one"} {
		mustIdea(t, root, "new", title)
	}
	mustIdea(t, root, "park", "2", "--reason", "later")
	mustIdea(t, root, "drop", "3", "--reason", "no appetite")
	mustIdea(t, root, "promote", "4", "--title", "Four")
	code, out, _ := ideaRun(t, root, "list")
	want := "1 [open] Open one\n2 [parked] Parked one — reason: later\n3 [dropped] Dropped one — reason: no appetite\n4 [promoted] Promoted one\n"
	if code != 0 || out != want {
		t.Fatalf("list = %d\n%s\nwant\n%s", code, out, want)
	}
	code, out, _ = ideaRun(t, root, "show", "2")
	if code != 0 || out != ideaFile(t, root, "ideas/2-parked-one/summary.md") || !strings.Contains(out, "status: parked") {
		t.Fatalf("show = %d, %q", code, out)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"show", "9"}, "idea 9 not found"},
		{[]string{"show", "abc"}, "invalid id: abc"},
		{[]string{"show", "0"}, "invalid id: 0"},
		{[]string{"show"}, "zumen idea show <id>"},
	} {
		if code, out, errOut := ideaRun(t, root, tc.args...); code != 1 || out != "" || !strings.Contains(errOut, tc.want) {
			t.Errorf("%q = %d, %q, %q; want 1 and %q", tc.args, code, out, errOut, tc.want)
		}
	}
}

// AC4, AC5: park, drop and reopen, with their refusals.
func TestIdeaParkDropReopen(t *testing.T) {
	setToday(t, "2026-11-12")
	root := workspace(t)
	mustIdea(t, root, "new", "An idea")
	summary := func() string { return ideaFile(t, root, "ideas/1-an-idea/summary.md") }
	logText := func() string { return ideaFile(t, root, "ideas/1-an-idea/log.md") }
	before := summary() + logText()

	for _, verb := range []string{"park", "drop", "reopen"} {
		for _, args := range [][]string{{verb, "1"}, {verb, "1", "--reason"}, {verb, "1", "--reason", "  "}, {verb, "--reason", "x"}, {verb, "1", "2", "--reason", "x"}} {
			if code, out, errOut := ideaRun(t, root, args...); code != 1 || out != "" || errOut == "" {
				t.Errorf("%q = %d, %q, %q; want a usage error", args, code, out, errOut)
			}
		}
	}
	if summary()+logText() != before {
		t.Fatal("a refused verb wrote something")
	}

	setToday(t, "2026-11-13")
	if code, out, _ := ideaRun(t, root, "park", "1", "--reason", "later"); code != 0 || out != "idea 1: parked\n" {
		t.Fatalf("park = %d, %q", code, out)
	}
	if s := summary(); !strings.Contains(s, "status: parked") || !strings.Contains(s, "reason: later") || !strings.Contains(s, "updated: 2026-11-13") {
		t.Errorf("summary after park:\n%s", s)
	}
	if !strings.HasSuffix(logText(), "- 2026-11-13 parked: later\n") {
		t.Errorf("log = %q", logText())
	}
	snapshot := summary() + logText()
	for _, verb := range []string{"park", "drop"} {
		if code, _, errOut := ideaRun(t, root, verb, "1", "--reason", "again"); code != 1 || !strings.Contains(errOut, "idea 1 is parked: only an open idea can be") {
			t.Errorf("%s of a parked idea = %d, %q", verb, code, errOut)
		}
	}
	if summary()+logText() != snapshot {
		t.Error("a refused verb wrote something")
	}
	if code, out, _ := ideaRun(t, root, "reopen", "1", "--reason", "back"); code != 0 || out != "idea 1: open\n" {
		t.Fatalf("reopen = %d, %q", code, out)
	}
	if code, _, errOut := ideaRun(t, root, "reopen", "1", "--reason", "again"); code != 1 || !strings.Contains(errOut, "idea 1 is open: only a parked or dropped idea can be reopened") {
		t.Errorf("reopen of an open idea = %d, %q", code, errOut)
	}
	if code, _, errOut := ideaRun(t, root, "park", "7", "--reason", "x"); code != 1 || !strings.Contains(errOut, "idea 7 not found") {
		t.Errorf("park of a missing idea = %d, %q", code, errOut)
	}
}

// AC7, AC8: promote creates a change as change new does, with the idea's summary
// as the intake, and can be repeated.
func TestIdeaPromote(t *testing.T) {
	setToday(t, "2026-11-12")
	root := workspace(t) // 1-auth-flow, intake 1
	mustIdea(t, root, "new", "Faster imports")
	summaryBefore := ideaFile(t, root, "ideas/1-faster-imports/summary.md")
	body := summaryBefore[strings.Index(summaryBefore, "\n---\n")+len("\n---\n"):]

	setToday(t, "2026-11-14")
	code, out, errOut := ideaRun(t, root, "promote", "1", "--title", "Part one", "--repo", "zumen")
	if code != 0 || !strings.Contains(out, "created change 2-part-one (source #2)") || !strings.HasSuffix(out, "idea 1: promoted\n") {
		t.Fatalf("promote = %d, %q, %q", code, out, errOut)
	}
	intake := ideaFile(t, root, "00_intake/2.md")
	wantHead := "---\nid: 2\ntitle: Part one\nsource: repo\nrepo: zumen\nidea: 1\n---\n\nPromoted from idea `ideas/1-faster-imports/` on 2026-11-14.\n\n"
	if !strings.HasPrefix(intake, wantHead) || !strings.Contains(intake, "## Problem") {
		t.Fatalf("intake =\n%s", intake)
	}
	if _, err := os.Stat(state.ChangeDir(root, "2-part-one")); err != nil {
		t.Fatalf("the change folder wasn't made: %v", err)
	}
	summary := ideaFile(t, root, "ideas/1-faster-imports/summary.md")
	for _, want := range []string{"status: promoted", "changes: 2-part-one", "updated: 2026-11-14"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary lacks %q:\n%s", want, summary)
		}
	}
	if got := summary[strings.Index(summary, "\n---\n")+len("\n---\n"):]; got != body {
		t.Error("promote changed the summary's body")
	}
	if log := ideaFile(t, root, "ideas/1-faster-imports/log.md"); !strings.HasSuffix(log, "- 2026-11-14 promoted to 2-part-one\n") {
		t.Errorf("log = %q", log)
	}

	// A split: a second change, and no --repo, so an issue intake.
	if code, _, errOut := ideaRun(t, root, "promote", "1", "--title", "Part two"); code != 0 {
		t.Fatalf("second promote = %d, %q", code, errOut)
	}
	if s := ideaFile(t, root, "ideas/1-faster-imports/summary.md"); !strings.Contains(s, "changes: 2-part-one, 3-part-two") {
		t.Errorf("summary after a split:\n%s", s)
	}
	if intake := ideaFile(t, root, "00_intake/3.md"); !strings.Contains(intake, "source: issue") || !strings.Contains(intake, "idea: 1\n") {
		t.Errorf("second intake:\n%s", intake)
	}
}

func TestIdeaPromoteRefusals(t *testing.T) {
	setToday(t, "2026-11-12")
	root := workspace(t)
	mustIdea(t, root, "new", "One")
	mustIdea(t, root, "new", "Two")
	mustIdea(t, root, "park", "1", "--reason", "later")
	mustIdea(t, root, "drop", "2", "--reason", "no")
	stateBefore, _ := os.ReadFile(state.StatePath(root))
	intakesBefore, _ := os.ReadDir(filepath.Join(root, "00_intake"))
	for _, id := range []string{"1", "2"} {
		code, out, errOut := ideaRun(t, root, "promote", id, "--title", "X")
		if code != 1 || out != "" || !strings.Contains(errOut, "reopen it first") {
			t.Errorf("promote of idea %s = %d, %q, %q", id, code, out, errOut)
		}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"promote", "1"}, "zumen idea promote <id>"},
		{[]string{"promote", "--title", "X"}, "zumen idea promote <id>"},
		{[]string{"promote", "1", "--title"}, "unknown flag"},
		{[]string{"promote", "9", "--title", "X"}, "idea 9 not found"},
		{[]string{"promote", "abc", "--title", "X"}, "invalid id: abc"},
		{[]string{"promote", "1", "--title", "X", "--bogus"}, "unknown flag"},
		{[]string{"promote", "1", "2", "--title", "X"}, "zumen idea promote <id>"},
	} {
		if code, out, errOut := ideaRun(t, root, tc.args...); code != 1 || out != "" || !strings.Contains(errOut, tc.want) {
			t.Errorf("%q = %d, %q, %q; want 1 and %q", tc.args, code, out, errOut, tc.want)
		}
	}
	if got, _ := os.ReadFile(state.StatePath(root)); string(got) != string(stateBefore) {
		t.Error("a refused promote changed STATE.json")
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "00_intake")); len(entries) != len(intakesBefore) {
		t.Errorf("a refused promote wrote an intake: %v", entries)
	}
}

// AC15: a promoted intake gets an id above every intake a change records, even
// when that intake's file was deleted.
func TestIdeaPromoteHonoursTheIntakeFloor(t *testing.T) {
	root := workspace(t)
	if code := runChange(root, []string{"new", "Second"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	if err := os.Remove(filepath.Join(root, "00_intake", "2.md")); err != nil {
		t.Fatal(err)
	}
	mustIdea(t, root, "new", "An idea")
	mustIdea(t, root, "promote", "1", "--title", "Third")
	if _, err := os.Stat(filepath.Join(root, "00_intake", "2.md")); err == nil {
		t.Fatal("the promoted intake took the deleted intake's id")
	}
	if intake := ideaFile(t, root, "00_intake/3.md"); !strings.Contains(intake, "idea: 1") {
		t.Fatalf("00_intake/3.md = %q, want the promoted intake", intake)
	}
}

// If recording the change on the idea fails, the change stays, and the error says
// how to finish.
func TestIdeaPromoteReportsAFailedIdeaUpdate(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	root := workspace(t)
	mustIdea(t, root, "new", "An idea")
	summary := filepath.Join(root, "ideas", "1-an-idea", "summary.md")
	if err := os.Chmod(summary, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(summary, 0o644) })
	code, out, errOut := ideaRun(t, root, "promote", "1", "--title", "Kept")
	if code != 1 || !strings.Contains(out, "created change 2-kept") {
		t.Fatalf("promote = %d, %q, %q", code, out, errOut)
	}
	if !strings.Contains(errOut, "created change 2-kept, but couldn't update idea 1") || !strings.Contains(errOut, "by hand") {
		t.Errorf("error = %q", errOut)
	}
	changes, err := state.ReadChanges(root)
	if err != nil || changes["2-kept"] == nil {
		t.Fatalf("the change should exist: %v, %v", changes, err)
	}
	if ideaFile(t, root, "00_intake/2.md") == "" {
		t.Error("the intake should exist")
	}
}

func TestIdeaNoVerbOrAnUnknownOne(t *testing.T) {
	root := workspace(t)
	for _, args := range [][]string{nil, {"bogus"}, {"list", "extra"}} {
		code, out, errOut := ideaRun(t, root, args...)
		if args == nil || args[0] == "bogus" {
			if code != 1 || out != "" || !strings.Contains(errOut, "zumen idea new <title>") {
				t.Errorf("%q = %d, %q, %q; want the group usage on stderr", args, code, out, errOut)
			}
		}
	}
	if code, _, errOut := ideaRun(t, root, "bogus"); !strings.Contains(errOut, `unknown verb "bogus"`) || code != 1 {
		t.Errorf("an unknown verb: %d, %q", code, errOut)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "ideas")); len(entries) != 0 {
		t.Errorf("a refused verb created %v", entries)
	}
}
