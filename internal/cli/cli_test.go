package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/kiso/repos"
	"github.com/btdstudio/zumen/internal/state"
)

// workspace returns an isolated root with one change at intake. It has no repo, so
// explore and synthesis start skipped.
func workspace(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	return root
}

func setGates(t *testing.T, root, value string) {
	t.Helper()
	data := []byte(`{"gates": "` + value + `"}`)
	if err := os.WriteFile(filepath.Join(state.Workspace(root), "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readLog(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(state.LogPath(root))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func TestGateRefusesSkippingAhead(t *testing.T) {
	root := workspace(t)
	if code := runGate(root, []string{"architecture", "approved"}); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Gates["architecture"] != "" || st.Phase != "intake" {
		t.Fatalf("refused gate changed state: %+v", st)
	}
}

func TestGateFollowsLoopInOrder(t *testing.T) {
	root := workspace(t)
	if code := runGate(root, []string{"intake", "approved"}); code != 0 {
		t.Fatal("intake")
	}
	if code := runGate(root, []string{"spec", "approved"}); code != 1 {
		t.Fatalf("spec before research = %d, want 1", code)
	}
	if code := runGate(root, []string{"research", "skipped", "--reason", "nothing to find out"}); code != 0 {
		t.Fatal("research")
	}
	for _, phase := range []string{"spec", "architecture", "breakdown"} {
		if code := runGate(root, []string{phase, "approved"}); code != 0 {
			t.Fatalf("gate %s = %d", phase, code)
		}
	}
	if log := readLog(t, root); strings.Contains(log, "override") {
		t.Fatalf("in-order run logged an override: %s", log)
	}
}

func TestGateForceOverridesAndLogs(t *testing.T) {
	root := workspace(t)
	if code := runGate(root, []string{"architecture", "approved", "--force"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if log := readLog(t, root); !strings.Contains(log, "override: zumen: architecture requires intake") {
		t.Fatalf("override not logged: %s", log)
	}
}

func TestGateNonBlockSettingWarnsAndProceeds(t *testing.T) {
	root := workspace(t)
	setGates(t, root, "auto")
	if code := runGate(root, []string{"architecture", "approved"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if log := readLog(t, root); !strings.Contains(log, "override:") {
		t.Fatalf("override not logged: %s", log)
	}
}

func TestGateRejectsBadNamesEvenWithForce(t *testing.T) {
	root := workspace(t)
	before := readLog(t, root)
	if code := runGate(root, []string{"architecture", "nope", "--force"}); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if log := readLog(t, root); log != before {
		t.Fatalf("log = %q", log)
	}
}

func TestChangeNewRejectsUnquotedTitle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if code := runChange(root, []string{"new", "Auth", "flow"}); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if _, err := os.Stat(state.SpecsDir(root)); err == nil {
		t.Fatal("rejected change created specs/")
	}
}

func TestChangeNewLeavesNothingBehindOnCorruptState(t *testing.T) {
	root := workspace(t)
	if err := os.WriteFile(state.StatePath(root), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runChange(root, []string{"new", "Second"}); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if _, err := os.Stat(filepath.Join(root, "00_intake", "2.md")); err == nil {
		t.Fatal("wrote an orphan intake")
	}
	if got, _ := os.ReadFile(state.StatePath(root)); string(got) != "{" {
		t.Fatalf("STATE.json overwritten: %s", got)
	}
}

func TestSourceAndAdrNewRejectStrayArguments(t *testing.T) {
	root := workspace(t)
	if code := runSource(root, []string{"new", "--title", "Auth", "flow"}); code != 1 {
		t.Fatalf("source new = %d, want 1", code)
	}
	if code := runAdr(root, []string{"new", "--title", "Use", "JWT"}); code != 1 {
		t.Fatalf("adr new = %d, want 1", code)
	}
}

func TestChangeNewWithRepoRequiresExplore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if code := runChange(root, []string{"new", "Survey", "--repo", "/some/repo"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	if code := runGate(root, []string{"intake", "approved"}); code != 0 {
		t.Fatalf("gate intake = %d", code)
	}
	if code := runGate(root, []string{"research", "skipped", "--reason", "none"}); code != 0 {
		t.Fatal("research")
	}
	if code := runGate(root, []string{"spec", "draft"}); code != 1 {
		t.Fatalf("spec before explore = %d, want 1", code)
	}
	if code := runGate(root, []string{"explore", "skipped"}); code != 1 {
		t.Fatalf("skip without reason = %d, want 1", code)
	}
	if code := runGate(root, []string{"explore", "skipped", "--reason", "already surveyed"}); code != 0 {
		t.Fatalf("skip with reason = %d", code)
	}
	if log := readLog(t, root); !strings.Contains(log, "explore → skipped (already surveyed)") {
		t.Fatalf("skip reason not logged: %s", log)
	}
	if code := runGate(root, []string{"spec", "draft"}); code != 0 {
		t.Fatalf("spec after skip = %d", code)
	}
}

func TestReopenClearsLaterApprovals(t *testing.T) {
	root := workspace(t)
	if code := runGate(root, []string{"intake", "approved"}); code != 0 {
		t.Fatal("intake")
	}
	if code := runGate(root, []string{"research", "skipped", "--reason", "none"}); code != 0 {
		t.Fatal("research")
	}
	for _, phase := range []string{"spec", "architecture"} {
		if code := runGate(root, []string{phase, "approved"}); code != 0 {
			t.Fatalf("gate %s = %d", phase, code)
		}
	}
	if code := runReopen(root, []string{"spec"}); code != 1 {
		t.Fatalf("reopen without reason = %d, want 1", code)
	}
	if code := runReopen(root, []string{"spec", "--reason", "scope changed"}); code != 0 {
		t.Fatalf("reopen = %d", code)
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Gates["spec"] != "draft" || st.Gates["architecture"] != "" || st.Gates["intake"] != "approved" || st.Gates["explore"] != "skipped" {
		t.Fatalf("gates = %+v", st.Gates)
	}
	if log := readLog(t, root); !strings.Contains(log, "reopened spec (scope changed)") {
		t.Fatalf("reopen not logged: %s", log)
	}
}

func TestChangeCloseRefusesGatesEvenForced(t *testing.T) {
	root := workspace(t)
	if code := runChange(root, []string{"close", "1-auth-flow"}); code != 1 {
		t.Fatalf("close without reason = %d, want 1", code)
	}
	if code := runChange(root, []string{"close", "1-auth-flow", "--reason", "dropped"}); code != 0 {
		t.Fatalf("close = %d", code)
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "" {
		t.Fatalf("closed change still active: %+v", st)
	}
	if code := runSwitch(root, []string{"1-auth-flow"}); code != 0 {
		t.Fatalf("switch = %d", code)
	}
	if code := runGate(root, []string{"intake", "approved", "--force"}); code != 1 {
		t.Fatalf("gate on closed = %d, want 1", code)
	}
}

func TestSwitchHidesClosedUnlessAll(t *testing.T) {
	root := workspace(t)
	if code := runChange(root, []string{"close", "1-auth-flow", "--reason", "dropped"}); code != 0 {
		t.Fatalf("close = %d", code)
	}
	list := func(args ...string) string {
		t.Helper()
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		orig := os.Stdout
		os.Stdout = w
		code := runSwitch(root, args)
		os.Stdout = orig
		w.Close()
		out, _ := io.ReadAll(r)
		if code != 0 {
			t.Fatalf("switch = %d", code)
		}
		return string(out)
	}
	if out := list(); strings.Contains(out, "1-auth-flow") {
		t.Fatalf("closed change listed: %q", out)
	}
	if out := list("--all"); !strings.Contains(out, "1-auth-flow") {
		t.Fatalf("--all hid the closed change: %q", out)
	}
}

func captureStdout(t *testing.T, fn func() int) (int, string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	code := fn()
	os.Stdout = orig
	w.Close()
	out, _ := io.ReadAll(r)
	return code, string(out)
}

func TestEnvPrintsFlagsForTheChangesRepos(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	first, second := t.TempDir(), t.TempDir()
	for name, path := range map[string]string{"first": first, "second": second, "other": t.TempDir()} {
		if code := repoCmd(root, "add", name, "https://github.com/org/"+name, "--path", path); code != 0 {
			t.Fatalf("repo add %s = %d", name, code)
		}
	}
	if code := runChange(root, []string{"new", "Release", "--repo", "first", "--repo", "second"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Gates["explore"] != "" || len(st.Repos) != 2 {
		t.Fatalf("st = %+v", st)
	}
	code, out := captureStdout(t, func() int { return runEnv(root) })
	if code != 0 || strings.TrimSpace(out) != "--add-dir "+first+" --add-dir "+second {
		t.Fatalf("code = %d, out = %q", code, out)
	}
}

func TestChangeNewRepoMayStillBeAPathButNotInAMultiRepoChange(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if code := runChange(root, []string{"new", "One", "--repo", "/some/path"}); code != 0 {
		t.Fatalf("single path = %d", code)
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Repos) != 0 || st.Gates["explore"] != "" {
		t.Fatalf("st = %+v", st)
	}
	if code := runChange(root, []string{"new", "Two", "--repo", "/a", "--repo", "/b"}); code != 1 {
		t.Fatalf("two paths = %d, want 1", code)
	}
	if _, err := os.Stat(filepath.Join(root, "00_intake", "2.md")); err == nil {
		t.Fatal("refused change wrote an intake")
	}
}

func TestEnvNeedsAnActiveChange(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if code := runEnv(t.TempDir()); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
}

func repoCmd(root string, args ...string) int {
	return repos.Run(root, "zumen", args, os.Stdout, os.Stderr)
}

func ticketRepo(t *testing.T, root string) {
	t.Helper()
	if code := repoCmd(root, "add", "app", "https://github.com/org/app"); code != 0 {
		t.Fatalf("repo add = %d", code)
	}
}

func TestTicketsNewCheckOrderStatus(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	if code := runTickets(root, []string{"new", "story", "No repo"}); code != 1 {
		t.Fatalf("story without repo = %d, want 1", code)
	}
	if code := runTickets(root, []string{"new", "story", "Bad repo", "--repo", "nope"}); code != 1 {
		t.Fatalf("unregistered repo = %d, want 1", code)
	}
	if code := runTickets(root, []string{"new", "bug", "Bad type"}); code != 1 {
		t.Fatalf("bad type = %d, want 1", code)
	}
	for _, c := range [][]string{
		{"new", "epic", "Release"},
		{"new", "feature", "Pipeline", "--parent", "E-0001"},
		{"new", "story", "Second", "--parent", "F-0002", "--repo", "app", "--depends", "S-0004"},
		{"new", "story", "First", "--parent", "F-0002", "--repo", "app"},
	} {
		if code := runTickets(root, c); code != 0 {
			t.Fatalf("tickets %v = %d", c, code)
		}
	}
	data, err := os.ReadFile(filepath.Join(state.ChangeDir(root, "1-auth-flow"), "tickets", "S-0003.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"change: 1-auth-flow", "spec: specs/1-auth-flow/functional-spec.md", "## Acceptance criteria", "status: draft"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("ticket missing %q:\n%s", want, data)
		}
	}
	code, out := captureStdout(t, func() int { return runTickets(root, []string{"check"}) })
	if code != 0 || !strings.Contains(out, "4 tickets ok") {
		t.Fatalf("check = %d: %s", code, out)
	}
	code, out = captureStdout(t, func() int { return runTickets(root, []string{"order"}) })
	if code != 0 || strings.Index(out, "S-0004") > strings.Index(out, "S-0003") || strings.Index(out, "E-0001") != 0 {
		t.Fatalf("order = %d:\n%s", code, out)
	}
	code, out = captureStdout(t, func() int { return runTickets(root, []string{"status"}) })
	if code != 0 || !strings.Contains(out, "draft      4") {
		t.Fatalf("status = %d:\n%s", code, out)
	}
}

func TestTicketsCheckCatchesCyclesAndUnreadableFiles(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	if code := runTickets(root, []string{"new", "story", "A", "--repo", "app", "--depends", "S-0002"}); code != 0 {
		t.Fatal("A")
	}
	if code := runTickets(root, []string{"new", "story", "B", "--repo", "app", "--depends", "S-0001"}); code != 0 {
		t.Fatal("B")
	}
	code, out := captureStdout(t, func() int { return runTickets(root, []string{"check"}) })
	if code != 1 || !strings.Contains(out, "cycle among S-0001, S-0002") {
		t.Fatalf("check = %d: %s", code, out)
	}
	if code := runTickets(root, []string{"order"}); code != 1 {
		t.Fatalf("order on a cycle = %d, want 1", code)
	}
	bad := filepath.Join(state.ChangeDir(root, "1-auth-flow"), "tickets", "S-0009.md")
	if err := os.WriteFile(bad, []byte("not a ticket"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = captureStdout(t, func() int { return runTickets(root, []string{"check"}) })
	if code != 1 || !strings.Contains(out, "S-0009.md") {
		t.Fatalf("check = %d: %s", code, out)
	}
	if code := runTickets(root, []string{"new", "story", "C", "--repo", "app"}); code != 1 {
		t.Fatalf("new with an unreadable ticket = %d, want 1", code)
	}
}

func TestTicketsPublishUnfreezeAndDrift(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	if code := runTickets(root, []string{"new", "story", "A", "--repo", "app"}); code != 0 {
		t.Fatal("new")
	}
	path := filepath.Join(state.ChangeDir(root, "1-auth-flow"), "tickets", "S-0001.md")
	if code := runTickets(root, []string{"mark-published", "S-0001", "--external", "ado=1"}); code != 1 {
		t.Fatalf("publishing a draft = %d, want 1", code)
	}
	draft, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(draft), "status: draft", "status: ready", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"mark-published", "S-0001"},
		{"mark-published", "S-0001", "--external", "noequals"},
		{"mark-published", "S-0099", "--external", "ado=1"},
		{"unfreeze", "S-0001"},
	} {
		if code := runTickets(root, args); code != 1 {
			t.Errorf("tickets %v = %d, want 1", args, code)
		}
	}
	if code := runTickets(root, []string{"mark-published", "S-0001", "--external", "github=org/app#7"}); code != 0 {
		t.Fatal("publish")
	}
	if log := readLog(t, root); !strings.Contains(log, "S-0001 published to github (org/app#7)") {
		t.Fatalf("log = %s", log)
	}
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "AC1", "AC9", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := captureStdout(t, func() int { return runTickets(root, []string{"check"}) })
	if code != 1 || !strings.Contains(out, "drifted") {
		t.Fatalf("check = %d: %s", code, out)
	}
	if code := runTickets(root, []string{"mark-published", "S-0001", "--external", "github=org/app#7"}); code != 1 {
		t.Fatal("publishing a drifted ticket should fail")
	}
	if code := runTickets(root, []string{"unfreeze", "S-0001", "--reason", "scope changed"}); code != 0 {
		t.Fatal("unfreeze")
	}
	code, out = captureStdout(t, func() int { return runTickets(root, []string{"status"}) })
	if code != 0 || !strings.Contains(out, "unfrozen   1  S-0001") {
		t.Fatalf("status = %d:\n%s", code, out)
	}
	if code := runTickets(root, []string{"mark-published", "S-0001", "--external", "github=org/app#7"}); code != 0 {
		t.Fatal("republish")
	}
	if code, _ := captureStdout(t, func() int { return runTickets(root, []string{"check"}) }); code != 0 {
		t.Fatalf("check after republish = %d", code)
	}
}

func TestTicketsNewRefusesWithoutAnOpenChange(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	if code := runChange(root, []string{"close", "1-auth-flow", "--reason", "x"}); code != 0 {
		t.Fatal("close")
	}
	if code := runTickets(root, []string{"new", "epic", "A"}); code != 1 {
		t.Fatalf("no active = %d, want 1", code)
	}
	if code := runSwitch(root, []string{"1-auth-flow"}); code != 0 {
		t.Fatal("switch")
	}
	if code := runTickets(root, []string{"new", "epic", "A"}); code != 1 {
		t.Fatalf("closed = %d, want 1", code)
	}
}
