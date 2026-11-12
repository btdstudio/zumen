package cli

import (
	"io"
	"os"
	"path/filepath"
	"slices"
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
	writeArtifacts(t, root)
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
		gate := func() int { return runGate(root, []string{phase, "approved"}) }
		if code := withStandInTicket(t, root, gate); code != 0 {
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

// AC2: a closed change whose folder was removed still keeps its id.
func TestChangeNewSkipsRecordedChangeIds(t *testing.T) {
	root := workspace(t) // 1-auth-flow
	if code := runChange(root, []string{"close", "1-auth-flow", "--reason", "done"}); code != 0 {
		t.Fatalf("close = %d", code)
	}
	if err := os.RemoveAll(state.ChangeDir(root, "1-auth-flow")); err != nil {
		t.Fatal(err)
	}
	if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	changes, err := state.ReadChanges(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := changes["2-auth-flow"]; !ok {
		t.Fatalf("changes = %v, want 2-auth-flow", changes)
	}
	if changes["1-auth-flow"].Status != "closed" {
		t.Fatalf("1-auth-flow = %q, want it still closed", changes["1-auth-flow"].Status)
	}
}

// intakeIDs lists the ids of the intake files under root.
func intakeIDs(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "00_intake"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range entries {
		ids = append(ids, e.Name())
	}
	return ids
}

// AC6: source new skips an intake id a change records, even with the file gone.
func TestSourceNewSkipsRecordedIntakeIds(t *testing.T) {
	root := workspace(t) // 1-auth-flow cites 00_intake/1.md
	if _, err := state.SeedChange(root, "2-gone", false, []string{"00_intake/5.md", "issue:40"}); err != nil {
		t.Fatal(err)
	}
	if code := runSource(root, []string{"new", "--title", "Next"}); code != 0 {
		t.Fatalf("source new = %d", code)
	}
	if got := intakeIDs(t, root); !slices.Equal(got, []string{"1.md", "6.md"}) {
		t.Fatalf("intake files = %v, want 1.md and 6.md", got)
	}
}

// Closed changes' sources count too: closing a change doesn't free its intake id.
func TestIntakeFloorCountsClosedChanges(t *testing.T) {
	root := workspace(t) // 1-auth-flow cites 00_intake/1.md
	if code := runChange(root, []string{"new", "Second"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	if code := runChange(root, []string{"close", "2-second", "--reason", "done"}); code != 0 {
		t.Fatalf("close = %d", code)
	}
	if err := os.Remove(filepath.Join(root, "00_intake", "2.md")); err != nil {
		t.Fatal(err)
	}
	if code := runSource(root, []string{"new", "--title", "Next"}); code != 0 {
		t.Fatalf("source new = %d", code)
	}
	if got := intakeIDs(t, root); !slices.Equal(got, []string{"1.md", "3.md"}) {
		t.Fatalf("intake files = %v, want 1.md and 3.md", got)
	}
}

// When seeding fails after the intake is written, change new removes the
// intake, so no file is left that no change records.
func TestChangeNewRemovesIntakeWhenSeedFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := workspace(t)
	specs := state.SpecsDir(root)
	// specs/ stays readable, so NextID works, but Seed can't make the folder.
	if err := os.Chmod(specs, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(specs, 0o755) })
	if code := runChange(root, []string{"new", "Second"}); code != 1 {
		t.Fatalf("change new = %d, want 1", code)
	}
	if got := intakeIDs(t, root); !slices.Equal(got, []string{"1.md"}) {
		t.Fatalf("intake files = %v, want only 1.md", got)
	}
}

// AC7: when files and records agree, or there's no STATE.json, ids are as before.
func TestIntakeIdsUnchangedWhenConsistent(t *testing.T) {
	root := workspace(t)
	if code := runSource(root, []string{"new", "--title", "Next"}); code != 0 {
		t.Fatalf("source new = %d", code)
	}
	if got := intakeIDs(t, root); !slices.Equal(got, []string{"1.md", "2.md"}) {
		t.Fatalf("consistent: intake files = %v, want 1.md and 2.md", got)
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	bare := t.TempDir()
	for _, title := range []string{"One", "Two"} {
		if code := runSource(bare, []string{"new", "--title", title}); code != 0 {
			t.Fatalf("source new %s = %d", title, code)
		}
	}
	if got := intakeIDs(t, bare); !slices.Equal(got, []string{"1.md", "2.md"}) {
		t.Fatalf("no STATE.json: intake files = %v, want 1.md and 2.md", got)
	}
}

// AC8: an unparseable STATE.json stops source new before it writes anything.
func TestSourceNewRefusesCorruptState(t *testing.T) {
	root := workspace(t)
	if err := os.WriteFile(state.StatePath(root), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stderr := captureStderr(t, func() int { return runSource(root, []string{"new", "--title", "Next"}) })
	if code != 1 || !strings.Contains(stderr, "cannot parse") {
		t.Fatalf("source new = %d, %q; want 1 and the parse error", code, stderr)
	}
	if got := intakeIDs(t, root); !slices.Equal(got, []string{"1.md"}) {
		t.Fatalf("intake files = %v, want only 1.md", got)
	}
}

// AC5: change new skips an intake id a change records, even with the file gone.
func TestChangeNewSkipsRecordedIntakeIds(t *testing.T) {
	root := workspace(t) // 1-auth-flow cites 00_intake/1.md
	if code := runChange(root, []string{"new", "Second"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	if err := os.Remove(filepath.Join(root, "00_intake", "2.md")); err != nil {
		t.Fatal(err)
	}
	if code := runChange(root, []string{"new", "Third"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	changes, err := state.ReadChanges(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := changes["3-third"]; got == nil || !slices.Equal(got.Sources, []string{"00_intake/3.md"}) {
		t.Fatalf("3-third = %+v, want sources [00_intake/3.md]", got)
	}
	if got := changes["2-second"].Sources; !slices.Equal(got, []string{"00_intake/2.md"}) {
		t.Fatalf("2-second sources = %v, want them unchanged", got)
	}

	// A recorded intake past every file pushes the next id past it too.
	if _, err := state.SeedChange(root, "4-gone", false, []string{"00_intake/5.md"}); err != nil {
		t.Fatal(err)
	}
	if code := runChange(root, []string{"new", "Fifth"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	if got := intakeIDs(t, root); !slices.Equal(got, []string{"1.md", "3.md", "6.md"}) {
		t.Fatalf("intake files = %v, want 1.md, 3.md and 6.md", got)
	}
}

// AC9: when the change id can't be allocated, change new writes nothing.
func TestChangeNewWritesNothingWhenIdAllocationFails(t *testing.T) {
	root := workspace(t)
	stateBefore, _ := os.ReadFile(state.StatePath(root))
	// A file where the specs folder should be makes NextID's read fail.
	if err := os.RemoveAll(state.SpecsDir(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state.SpecsDir(root), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stderr := captureStderr(t, func() int { return runChange(root, []string{"new", "Second"}) })
	if code != 1 || stderr == "" {
		t.Fatalf("change new = %d, %q; want 1 and an error", code, stderr)
	}
	if got := intakeIDs(t, root); !slices.Equal(got, []string{"1.md"}) {
		t.Fatalf("intake files = %v, want only 1.md", got)
	}
	if after, _ := os.ReadFile(state.StatePath(root)); string(after) != string(stateBefore) {
		t.Fatal("STATE.json changed")
	}
}

func TestSourceAndAdrNewRejectStrayArguments(t *testing.T) {
	root := workspace(t)
	for _, run := range []struct {
		name string
		fn   func() int
	}{
		{"source new", func() int { return runSource(root, []string{"new", "--title", "Auth", "flow"}) }},
		{"source new", func() int { return runSource(root, []string{"new", "Auth flow"}) }},
		{"adr new", func() int { return runAdr(root, []string{"new", "--title", "Use", "JWT"}) }},
		{"adr new", func() int { return runAdr(root, []string{"new", "Use JWT"}) }},
	} {
		code, stderr := captureStderr(t, run.fn)
		if code != 1 {
			t.Fatalf("%s = %d, want 1", run.name, code)
		}
		if !strings.Contains(stderr, "--title") || strings.Contains(stderr, "quote") {
			t.Fatalf("%s error should name --title, not quoting: %q", run.name, stderr)
		}
	}
}

// A mistyped flag, or a flag with no value, is reported as a flag problem, not
// with --title advice.
func TestSourceAndAdrNewReportFlagProblems(t *testing.T) {
	root := workspace(t)
	for _, tc := range []struct {
		args []string
		bad  string
		fn   func(string, []string) int
	}{
		{[]string{"new", "--title", "X", "--decider", "bob"}, "--decider", runAdr},
		{[]string{"new", "--title"}, "--title", runAdr},
		{[]string{"new", "--tittle", "X"}, "--tittle", runSource},
		{[]string{"new", "--title", "X", "--body"}, "--body", runSource},
	} {
		code, stderr := captureStderr(t, func() int { return tc.fn(root, tc.args) })
		if code != 1 {
			t.Fatalf("%q = %d, want 1", tc.args, code)
		}
		if !strings.Contains(stderr, "unknown flag \""+tc.bad+"\"") || strings.Contains(stderr, "use --title") {
			t.Fatalf("%q error = %q, want an unknown-flag error for %s", tc.args, stderr, tc.bad)
		}
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
	writeArtifacts(t, root)
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
	fillCriteria(t, root, "S-0001")
	startPublish(t, root)
	if code := runTickets(root, []string{"mark-published", "S-0001", "--external", "ado=1"}); code != 1 {
		t.Fatalf("publishing a draft = %d, want 1", code)
	}
	if code := runTickets(root, []string{"ready", "S-0001"}); code != 0 {
		t.Fatal("ready")
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

// approveThrough gates the workspace's change in order up to and including phase:
// intake approved, research skipped, then spec, architecture and breakdown
// approved. Explore and synthesis already start skipped.
func approveThrough(t *testing.T, root, phase string) {
	t.Helper()
	writeArtifacts(t, root)
	steps := [][]string{
		{"intake", "approved"},
		{"research", "skipped", "--reason", "none needed"},
		{"spec", "approved"},
		{"architecture", "approved"},
		{"breakdown", "approved"},
	}
	for _, s := range steps {
		run := func() int { return runGate(root, s) }
		gate := run
		if s[0] == "breakdown" {
			gate = func() int { return withStandInTicket(t, root, run) }
		}
		if code := gate(); code != 0 {
			t.Fatalf("gate %v = %d", s, code)
		}
		if s[0] == phase {
			return
		}
	}
	t.Fatalf("unknown phase %q", phase)
}

// testSpec and testArchitecture pass the spec and architecture artifact checks.
const testSpec = "# Functional spec — Auth flow\n\n## Functional requirements\n\n- R1. Sign in. — **must**\n"

const testArchitecture = "# Architecture — Auth flow\n\n## Diagrams\n\n" +
	"### Context\n\n```mermaid\nflowchart LR\n  user --> app\n```\n\n" +
	"### Layers\n\n```mermaid\nflowchart TB\n  cli --> core\n```\n\n" +
	"### Flow: sign in\n\n```mermaid\nsequenceDiagram\n  user->>app: sign in\n```\n"

// activeChangeDir is the active change's folder.
func activeChangeDir(t *testing.T, root string) string {
	t.Helper()
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	return state.ChangeDir(root, st.Name)
}

// writeArtifacts gives the active change a spec and architecture that pass the
// gate checks, leaving existing files alone.
func writeArtifacts(t *testing.T, root string) {
	t.Helper()
	dir := activeChangeDir(t, root)
	for name, text := range map[string]string{"functional-spec.md": testSpec, "architecture.md": testArchitecture} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// withStandInTicket runs gate with a valid draft epic in a change that has no
// tickets yet, then removes it. Breakdown needs a ticket to approve, but many
// tests create theirs afterwards and expect their ids and counts unchanged.
// The epic is written directly, so it leaves no LOG.md line.
func withStandInTicket(t *testing.T, root string, gate func() int) int {
	t.Helper()
	dir := filepath.Join(activeChangeDir(t, root), "tickets")
	if entries, _ := filepath.Glob(filepath.Join(dir, "*.md")); len(entries) > 0 {
		return gate()
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		// The gate would commit the stand-in, and removing it would leave the
		// tree dirty: a git test creates its tickets before breakdown instead.
		t.Fatal("withStandInTicket in a git workspace: create a ticket before breakdown")
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "E-0001.md")
	text := "---\nschema: 1\nid: E-0001\ntype: epic\ntitle: Stand-in\nchange: " + st.Name + "\nstatus: draft\n---\n\n## Context\nA stand-in for the gate.\n"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(p)
	return gate()
}

// startPublish approves the workspace's change through breakdown and starts publish.
func startPublish(t *testing.T, root string) {
	t.Helper()
	approveThrough(t, root, "breakdown")
	if code := runGate(root, []string{"publish", "in-progress"}); code != 0 {
		t.Fatalf("publish in-progress = %d", code)
	}
}

// markPublished runs `tickets mark-published` with args and returns its stdout.
func markPublished(t *testing.T, root string, args ...string) (int, string) {
	t.Helper()
	return captureStdout(t, func() int { return runTickets(root, append([]string{"mark-published"}, args...)) })
}

func ticketPath(root, id string) string {
	return filepath.Join(state.ChangeDir(root, "1-auth-flow"), "tickets", id+".md")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// fillCriteria replaces the template's placeholder criterion with a real one.
func fillCriteria(t *testing.T, root, id string) {
	t.Helper()
	path := ticketPath(root, id)
	body := strings.Replace(readFile(t, path), "- [ ] AC1: Given … When … Then …", "- [ ] AC1: Given a user, When they log in, Then they see the dashboard", 1)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readyWorkspace has a story S-0001 with real criteria, with gates up to phase.
func readyWorkspace(t *testing.T, phase string) string {
	t.Helper()
	root := workspace(t)
	ticketRepo(t, root)
	writeArtifacts(t, root)
	if code := runTickets(root, []string{"new", "story", "Login", "--repo", "app", "--requirements", "R1"}); code != 0 {
		t.Fatal("new")
	}
	fillCriteria(t, root, "S-0001")
	approveThrough(t, root, phase)
	return root
}

func TestTicketsReadyBlockedUntilBreakdownApproved(t *testing.T) {
	root := readyWorkspace(t, "architecture")
	if code := runGate(root, []string{"breakdown", "await-approval"}); code != 0 {
		t.Fatal("breakdown await-approval")
	}
	before, logBefore := readFile(t, ticketPath(root, "S-0001")), readLog(t, root)
	code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0001"}) })
	if code == 0 || !strings.Contains(out, "breakdown") || !strings.Contains(out, "await-approval") {
		t.Fatalf("ready = %d: %s", code, out)
	}
	if after := readFile(t, ticketPath(root, "S-0001")); after != before {
		t.Fatalf("refused ready changed the file:\n%s", after)
	}
	if readLog(t, root) != logBefore {
		t.Fatal("refused ready wrote to LOG.md")
	}
}

func TestTicketsReadyForceOverridesAndLogs(t *testing.T) {
	root := readyWorkspace(t, "architecture")
	if code := runTickets(root, []string{"ready", "S-0001", "--force"}); code != 0 {
		t.Fatalf("ready --force = %d", code)
	}
	data := readFile(t, ticketPath(root, "S-0001"))
	if !strings.Contains(data, "status: ready") || !strings.Contains(data, "ready_hash: \"v2:") {
		t.Fatalf("ticket:\n%s", data)
	}
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"check"}) }); code != 0 {
		t.Fatalf("check = %d: %s", code, out)
	}
	log := readLog(t, root)
	if !strings.Contains(log, "1-auth-flow: override: zumen: tickets ready requires breakdown approved") || !strings.Contains(log, "1-auth-flow: S-0001 readied") {
		t.Fatalf("log = %s", log)
	}
}

func TestTicketsReadyAllIsAllOrNothing(t *testing.T) {
	root := readyWorkspace(t, "breakdown")
	if code := runTickets(root, []string{"new", "story", "Logout", "--repo", "app"}); code != 0 {
		t.Fatal("new")
	}
	one, two, logBefore := readFile(t, ticketPath(root, "S-0001")), readFile(t, ticketPath(root, "S-0002")), readLog(t, root)
	code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "--all"}) })
	if code != 1 || !strings.Contains(out, "S-0002") || !strings.Contains(out, "Given … When … Then …") {
		t.Fatalf("ready --all = %d: %s", code, out)
	}
	if readFile(t, ticketPath(root, "S-0001")) != one || readFile(t, ticketPath(root, "S-0002")) != two {
		t.Fatal("a refused --all changed a ticket")
	}
	if readLog(t, root) != logBefore {
		t.Fatal("a refused --all wrote to LOG.md")
	}
}

func TestTicketsReadyRequiresParent(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	for _, c := range [][]string{
		{"new", "feature", "Accounts"},
		{"new", "story", "Login", "--parent", "F-0001", "--repo", "app"},
	} {
		if code := runTickets(root, c); code != 0 {
			t.Fatalf("tickets %v = %d", c, code)
		}
	}
	approveThrough(t, root, "breakdown")
	// The orphan comes after breakdown: its missing parent would block the gate.
	if code := runTickets(root, []string{"new", "story", "Orphan", "--parent", "F-0099", "--repo", "app"}); code != 0 {
		t.Fatal("new Orphan")
	}
	for _, id := range []string{"F-0001", "S-0002", "S-0003"} {
		fillCriteria(t, root, id)
	}
	code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0002"}) })
	if code != 1 || !strings.Contains(out, "parent F-0001 is draft") {
		t.Fatalf("ready story alone = %d: %s", code, out)
	}
	code, out = captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0003"}) })
	if code != 1 || !strings.Contains(out, "parent F-0099 not found") {
		t.Fatalf("ready orphan = %d: %s", code, out)
	}
	code, out = captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0002", "F-0001"}) })
	if code != 0 || !strings.Contains(out, "S-0002: ready") || !strings.Contains(out, "F-0001: ready") {
		t.Fatalf("ready both = %d: %s", code, out)
	}
	if !strings.Contains(readFile(t, ticketPath(root, "F-0001")), "status: ready") {
		t.Fatal("feature not ready")
	}
}

func TestTicketsReadyPromptWarns(t *testing.T) {
	root := readyWorkspace(t, "architecture")
	setGates(t, root, "prompt")
	if code := runTickets(root, []string{"ready", "S-0001"}); code != 0 {
		t.Fatalf("ready under prompt = %d", code)
	}
	if !strings.Contains(readFile(t, ticketPath(root, "S-0001")), "status: ready") {
		t.Fatal("not readied")
	}
	if log := readLog(t, root); !strings.Contains(log, "override:") {
		t.Fatalf("log = %s", log)
	}
}

func TestTicketsReadyRefreshesAndRefusesPublished(t *testing.T) {
	root := readyWorkspace(t, "breakdown")
	if code := runTickets(root, []string{"ready", "S-0001"}); code != 0 {
		t.Fatal("ready")
	}
	path := ticketPath(root, "S-0001")
	edited := strings.Replace(readFile(t, path), "they see the dashboard", "they see their dashboard", 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"check"}) }); code != 1 || !strings.Contains(out, "changed since") {
		t.Fatalf("check after edit = %d: %s", code, out)
	}
	if code := runTickets(root, []string{"ready", "S-0001"}); code != 0 {
		t.Fatal("re-ready")
	}
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"check"}) }); code != 0 {
		t.Fatalf("check after re-ready = %d: %s", code, out)
	}
	if code := runGate(root, []string{"publish", "in-progress"}); code != 0 {
		t.Fatal("publish in-progress")
	}
	if code := runTickets(root, []string{"mark-published", "S-0001", "--external", "github=org/app#1"}); code != 0 {
		t.Fatal("publish")
	}
	published := readFile(t, path)
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0001"}) }); code != 1 || !strings.Contains(out, "published") {
		t.Fatalf("ready a published ticket = %d: %s", code, out)
	}
	if readFile(t, path) != published {
		t.Fatal("refused ready changed a published ticket")
	}
}

func TestTicketsReadyAllLogsOnce(t *testing.T) {
	root := readyWorkspace(t, "breakdown")
	if code := runTickets(root, []string{"new", "story", "Logout", "--repo", "app"}); code != 0 {
		t.Fatal("new")
	}
	fillCriteria(t, root, "S-0002")
	before := readLog(t, root)
	if code := runTickets(root, []string{"ready", "--all"}); code != 0 {
		t.Fatal("ready --all")
	}
	added := strings.TrimPrefix(readLog(t, root), before)
	if n := strings.Count(added, " readied"); n != 2 || strings.Count(added, "\n") != 2 || strings.Contains(added, "override") {
		t.Fatalf("log gained:\n%s", added)
	}
	for _, id := range []string{"S-0001", "S-0002"} {
		if !strings.Contains(readFile(t, ticketPath(root, id)), "status: ready") {
			t.Fatalf("%s not ready", id)
		}
	}
	code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "--all"}) })
	if code != 0 || !strings.Contains(out, "no draft tickets in 1-auth-flow") {
		t.Fatalf("ready --all with no drafts = %d: %s", code, out)
	}
}

func TestTicketsReadyUsageErrors(t *testing.T) {
	root := readyWorkspace(t, "breakdown")
	for _, args := range [][]string{
		{"ready"},
		{"ready", "--force"},
		{"ready", "S-0001", "--all"},
		{"ready", "S-0001", "--bogus"},
	} {
		if code := runTickets(root, args); code != 1 {
			t.Errorf("tickets %v = %d, want 1", args, code)
		}
	}
	code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0001", "S-0099"}) })
	if code != 1 || !strings.Contains(out, "S-0099: no such ticket") {
		t.Fatalf("unknown id = %d: %s", code, out)
	}
	if strings.Contains(readFile(t, ticketPath(root, "S-0001")), "status: ready") {
		t.Fatal("a refused run readied S-0001")
	}
}

func TestTicketsReadyRefusesWithAnUnreadableTicket(t *testing.T) {
	root := readyWorkspace(t, "breakdown")
	bad := ticketPath(root, "S-0002")
	if err := os.WriteFile(bad, []byte("---\nschema: [\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, ticketPath(root, "S-0001"))
	for _, args := range [][]string{{"ready", "--all"}, {"ready", "S-0001"}} {
		code, out := captureStdout(t, func() int { return runTickets(root, args) })
		if code != 1 || !strings.Contains(out, "S-0002.md") {
			t.Fatalf("tickets %v = %d: %s", args, code, out)
		}
	}
	if readFile(t, ticketPath(root, "S-0001")) != before {
		t.Fatal("a refused run readied S-0001")
	}
	if err := os.Remove(bad); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ticketPath(root, "S-0001"), []byte(strings.Replace(before, "status: draft", "status: ready", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("---\nschema: [\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "--all"}) })
	if code != 1 || strings.Contains(out, "no draft tickets") {
		t.Fatalf("--all with only an unreadable draft = %d: %s", code, out)
	}
}

// newReady makes a ticket with real criteria and readies it. Gates must allow it.
func newReady(t *testing.T, root, id string, args ...string) {
	t.Helper()
	// A ready story or task cites a requirement, or tickets check warns. R1 is
	// testSpec's, and citing it also covers it.
	if (args[0] == "story" || args[0] == "task") && !slices.Contains(args, "--requirements") {
		args = append(args, "--requirements", "R1")
	}
	if code := runTickets(root, append([]string{"new"}, args...)); code != 0 {
		t.Fatalf("tickets new %v = %d", args, code)
	}
	fillCriteria(t, root, id)
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", id}) }); code != 0 {
		t.Fatalf("ready %s = %d: %s", id, code, out)
	}
}

func TestTicketsMarkPublishedNeedsPublishGate(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	approveThrough(t, root, "breakdown")
	newReady(t, root, "S-0001", "story", "Login", "--repo", "app")
	before, logBefore := readFile(t, ticketPath(root, "S-0001")), readLog(t, root)
	code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1")
	if code != 1 || !strings.Contains(out, `requires publish in-progress for 1-auth-flow (it's "unset")`) {
		t.Fatalf("mark-published = %d: %s", code, out)
	}
	if readFile(t, ticketPath(root, "S-0001")) != before || readLog(t, root) != logBefore {
		t.Fatal("a refused mark-published changed the ticket or LOG.md")
	}
	if code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1", "--force"); code != 0 {
		t.Fatalf("mark-published --force = %d: %s", code, out)
	}
	if !strings.Contains(readFile(t, ticketPath(root, "S-0001")), "status: published") {
		t.Fatal("not published")
	}
	added := strings.TrimPrefix(readLog(t, root), logBefore)
	if !strings.Contains(added, "1-auth-flow: override: zumen: mark-published requires publish in-progress") || !strings.Contains(added, "1-auth-flow: S-0001 published to github (o/r#1)") {
		t.Fatalf("log gained:\n%s", added)
	}
}

func TestTicketsMarkPublishedPromptWarns(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	approveThrough(t, root, "breakdown")
	newReady(t, root, "S-0001", "story", "Login", "--repo", "app")
	setGates(t, root, "prompt")
	code, stderr := captureStderr(t, func() int {
		return runTickets(root, []string{"mark-published", "S-0001", "--external", "github=o/r#1"})
	})
	if code != 0 || !strings.Contains(stderr, "warning:") {
		t.Fatalf("mark-published under prompt = %d: %s", code, stderr)
	}
	if log := readLog(t, root); !strings.Contains(log, "override:") {
		t.Fatalf("log = %s", log)
	}
}

func TestTicketsMarkPublishedNeedsPublishedParent(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	startPublish(t, root)
	newReady(t, root, "F-0001", "feature", "Accounts")
	newReady(t, root, "S-0002", "story", "Login", "--parent", "F-0001", "--repo", "app")
	before := readFile(t, ticketPath(root, "S-0002"))
	code, out := markPublished(t, root, "S-0002", "--external", "github=o/r#2")
	if code != 1 || !strings.Contains(out, "S-0002: parent F-0001 has no github id") {
		t.Fatalf("story before its feature = %d: %s", code, out)
	}
	if readFile(t, ticketPath(root, "S-0002")) != before {
		t.Fatal("a refused mark-published changed the story")
	}
	if code, out := markPublished(t, root, "F-0001", "--external", "github=o/r#1"); code != 0 {
		t.Fatalf("feature = %d: %s", code, out)
	}
	if code, out := markPublished(t, root, "S-0002", "--external", "github=o/r#2"); code != 0 {
		t.Fatalf("story after its feature = %d: %s", code, out)
	}
}

func TestTicketsMarkPublishedChecksToolAcrossChanges(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	if code := runChange(root, []string{"new", "Billing"}); code != 0 {
		t.Fatal("change new Billing")
	}
	startPublish(t, root)
	if code := runTickets(root, []string{"new", "story", "Invoices", "--repo", "app"}); code != 0 {
		t.Fatal("new in Billing")
	}
	other := filepath.Join(state.ChangeDir(root, "2-billing"), "tickets", "S-0001.md")
	body := strings.Replace(readFile(t, other), "- [ ] AC1: Given … When … Then …", "- [ ] AC1: Given an order, When it ships, Then an invoice is sent", 1)
	if err := os.WriteFile(other, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runTickets(root, []string{"ready", "S-0001"}); code != 0 {
		t.Fatal("ready in Billing")
	}
	if code, out := markPublished(t, root, "S-0001", "--external", "ado=AB#1"); code != 0 {
		t.Fatalf("publish to ado = %d: %s", code, out)
	}
	if code := runSwitch(root, []string{"1-auth-flow"}); code != 0 {
		t.Fatal("switch")
	}
	startPublish(t, root)
	newReady(t, root, "S-0002", "story", "Login", "--repo", "app", "--depends", "S-0001")
	code, out := markPublished(t, root, "S-0002", "--external", "github=o/r#2")
	if code != 1 || !strings.Contains(out, "S-0002: depends_on S-0001 has no github id") {
		t.Fatalf("github with an ado-only predecessor = %d: %s", code, out)
	}
	if code, out := markPublished(t, root, "S-0002", "--external", "ado=AB#2"); code != 0 {
		t.Fatalf("ado with an ado predecessor = %d: %s", code, out)
	}
}

func TestTicketsMarkPublishedUnfrozenPredecessorCounts(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	startPublish(t, root)
	newReady(t, root, "S-0001", "story", "Login", "--repo", "app")
	newReady(t, root, "S-0002", "story", "Logout", "--repo", "app", "--depends", "S-0001")
	if code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1"); code != 0 {
		t.Fatalf("predecessor = %d: %s", code, out)
	}
	if code := runTickets(root, []string{"unfreeze", "S-0001", "--reason", "rework"}); code != 0 {
		t.Fatal("unfreeze")
	}
	if code, out := markPublished(t, root, "S-0002", "--external", "github=o/r#2"); code != 0 {
		t.Fatalf("after an unfrozen predecessor = %d: %s", code, out)
	}
}

func TestTicketsMarkPublishedNeedsValidReadyMark(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	startPublish(t, root)
	if code := runTickets(root, []string{"new", "story", "Login", "--repo", "app"}); code != 0 {
		t.Fatal("new")
	}
	fillCriteria(t, root, "S-0001")
	path := ticketPath(root, "S-0001")
	if code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1"); code != 1 || !strings.Contains(out, "run `zumen tickets ready S-0001` first") {
		t.Fatalf("draft = %d: %s", code, out)
	}
	byHand := strings.Replace(readFile(t, path), "status: draft", "status: ready", 1)
	if err := os.WriteFile(path, []byte(byHand), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1"); code != 1 || !strings.Contains(out, "not readied through the CLI") {
		t.Fatalf("ready by hand = %d: %s", code, out)
	}
	if readFile(t, path) != byHand {
		t.Fatal("a refused mark-published changed a hand-readied ticket")
	}
	if code := runTickets(root, []string{"ready", "S-0001"}); code != 0 {
		t.Fatal("ready")
	}
	edited := strings.Replace(readFile(t, path), "they see the dashboard", "they see their dashboard", 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1"); code != 1 || !strings.Contains(out, "changed since readied") {
		t.Fatalf("edited after ready = %d: %s", code, out)
	}
	if readFile(t, path) != edited {
		t.Fatal("a refused mark-published changed an edited ticket")
	}
}

func TestTicketsMarkPublishedForceDoesntSkipPredecessors(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	approveThrough(t, root, "breakdown")
	newReady(t, root, "S-0001", "story", "Login", "--repo", "app", "--depends", "S-0099")
	before := readFile(t, ticketPath(root, "S-0001"))
	code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1", "--force")
	if code != 1 || !strings.Contains(out, "S-0001: depends_on S-0099 not found") {
		t.Fatalf("forced with a missing predecessor = %d: %s", code, out)
	}
	if readFile(t, ticketPath(root, "S-0001")) != before {
		t.Fatal("a refused mark-published changed the ticket")
	}
}

func TestTicketsMarkPublishedChecksSupersedes(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	startPublish(t, root)
	newReady(t, root, "S-0001", "story", "Login v1", "--repo", "app")
	if code := runTickets(root, []string{"new", "story", "Login v2", "--repo", "app"}); code != 0 {
		t.Fatal("new")
	}
	path := ticketPath(root, "S-0002")
	if err := os.WriteFile(path, []byte(strings.Replace(readFile(t, path), "repo: app\n", "repo: app\nsupersedes: S-0001\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	fillCriteria(t, root, "S-0002")
	if code := runTickets(root, []string{"ready", "S-0002"}); code != 0 {
		t.Fatal("ready")
	}
	code, out := markPublished(t, root, "S-0002", "--external", "github=o/r#2")
	if code != 1 || !strings.Contains(out, "S-0002: supersedes S-0001 has no github id") {
		t.Fatalf("superseding an unpublished ticket = %d: %s", code, out)
	}
	if code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1"); code != 0 {
		t.Fatalf("superseded = %d: %s", code, out)
	}
	if code, out := markPublished(t, root, "S-0002", "--external", "github=o/r#2"); code != 0 {
		t.Fatalf("superseding = %d: %s", code, out)
	}
}

func TestTicketsMarkPublishedListsEveryProblem(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	approveThrough(t, root, "breakdown")
	newReady(t, root, "F-0001", "feature", "Accounts")
	newReady(t, root, "S-0002", "story", "Login", "--parent", "F-0001", "--repo", "app")
	code, out := markPublished(t, root, "S-0002", "--external", "github=o/r#2")
	if code != 1 || !strings.Contains(out, "requires publish in-progress") || !strings.Contains(out, "parent F-0001 has no github id") {
		t.Fatalf("mark-published = %d: %s", code, out)
	}
}

func TestTicketsMarkPublishedRefusesWithAnUnreadableTicket(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	startPublish(t, root)
	newReady(t, root, "S-0001", "story", "Login", "--repo", "app")
	if err := os.WriteFile(ticketPath(root, "S-0009"), []byte("---\nschema: [\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, ticketPath(root, "S-0001"))
	code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1")
	if code != 1 || !strings.Contains(out, "S-0009.md") {
		t.Fatalf("mark-published with an unreadable ticket = %d: %s", code, out)
	}
	if readFile(t, ticketPath(root, "S-0001")) != before {
		t.Fatal("a refused mark-published changed the ticket")
	}
}

func TestTicketsMarkPublishedRefusesClosedAndAmbiguous(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	startPublish(t, root)
	newReady(t, root, "S-0001", "story", "Login", "--repo", "app")
	closed := strings.Replace(readFile(t, ticketPath(root, "S-0001")), "status: ready", "status: closed", 1)
	if err := os.WriteFile(ticketPath(root, "S-0001"), []byte(closed), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1"); code != 1 || !strings.Contains(out, "can't publish a closed ticket") {
		t.Fatalf("closed = %d: %s", code, out)
	}
	if readFile(t, ticketPath(root, "S-0001")) != closed {
		t.Fatal("a refused mark-published changed a closed ticket")
	}

	newReady(t, root, "F-0002", "feature", "Accounts")
	newReady(t, root, "S-0003", "story", "Logout", "--parent", "F-0002", "--repo", "app")
	if code, out := markPublished(t, root, "F-0002", "--external", "github=o/r#2"); code != 0 {
		t.Fatalf("feature = %d: %s", code, out)
	}
	// A second F-0002 in another change makes the parent ambiguous.
	dup := filepath.Join(state.ChangeDir(root, "2-other"), "tickets", "F-0002.md")
	if err := os.MkdirAll(filepath.Dir(dup), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dup, []byte(strings.Replace(readFile(t, ticketPath(root, "F-0002")), "change: 1-auth-flow", "change: 2-other", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := markPublished(t, root, "S-0003", "--external", "github=o/r#3"); code != 1 || !strings.Contains(out, "parent F-0002 is ambiguous") {
		t.Fatalf("ambiguous parent = %d: %s", code, out)
	}
}

func checkOutput(t *testing.T, root string) (int, string) {
	t.Helper()
	return captureStdout(t, func() int { return runTickets(root, []string{"check"}) })
}

func reopen(t *testing.T, root, phase string) {
	t.Helper()
	if code := runReopen(root, []string{phase, "--reason", "rework"}); code != 0 {
		t.Fatalf("reopen %s = %d", phase, code)
	}
}

func TestTicketsCheckWarnsAfterBreakdownReopened(t *testing.T) {
	root := readyWorkspace(t, "breakdown")
	newReady(t, root, "S-0002", "story", "Logout", "--repo", "app")
	if code := runTickets(root, []string{"ready", "S-0001"}); code != 0 {
		t.Fatal("ready")
	}
	if code, out := checkOutput(t, root); code != 0 || strings.Contains(out, "!") {
		t.Fatalf("check before reopen = %d: %s", code, out)
	}
	reopen(t, root, "breakdown")
	code, out := checkOutput(t, root)
	for _, want := range []string{
		`! S-0001: ready, but breakdown for 1-auth-flow is "draft"`,
		`! S-0002: ready, but breakdown for 1-auth-flow is "draft"`,
		"2 tickets ok, 2 warning(s)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("check output lacks %q", want)
		}
	}
	if code != 0 {
		t.Fatalf("check with only warnings = %d, want 0:\n%s", code, out)
	}
}

func TestTicketsCheckWarningsDontHideErrors(t *testing.T) {
	root := readyWorkspace(t, "breakdown")
	if code := runTickets(root, []string{"ready", "S-0001"}); code != 0 {
		t.Fatal("ready")
	}
	reopen(t, root, "breakdown")
	if err := os.WriteFile(ticketPath(root, "S-0009"), []byte("not a ticket"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := checkOutput(t, root)
	errAt, warnAt := strings.Index(out, "✗ "), strings.Index(out, "! S-0001")
	if code != 1 || errAt < 0 || warnAt < 0 || errAt > warnAt || strings.Contains(out, "tickets ok") {
		t.Fatalf("check with an error and a warning = %d:\n%s", code, out)
	}
}

func TestTicketsCheckWarnsUnsetAfterEarlierReopen(t *testing.T) {
	root := readyWorkspace(t, "breakdown")
	if code := runTickets(root, []string{"ready", "S-0001"}); code != 0 {
		t.Fatal("ready")
	}
	reopen(t, root, "spec")
	code, out := checkOutput(t, root)
	if code != 0 || !strings.Contains(out, `! S-0001: ready, but breakdown for 1-auth-flow is "unset"`) {
		t.Fatalf("check after reopen spec = %d:\n%s", code, out)
	}
}

func TestTicketsCheckNoWarningWhenApprovedOrNotReady(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	startPublish(t, root)
	newReady(t, root, "S-0001", "story", "Published", "--repo", "app")
	if code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1"); code != 0 {
		t.Fatalf("publish = %d: %s", code, out)
	}
	newReady(t, root, "S-0002", "story", "Ready", "--repo", "app")
	if code := runTickets(root, []string{"new", "story", "Draft", "--repo", "app"}); code != 0 {
		t.Fatal("new draft")
	}
	reopen(t, root, "breakdown")
	code, out := checkOutput(t, root)
	if code != 0 || strings.Count(out, "! ") != 1 || !strings.Contains(out, "! S-0002: ready") || !strings.Contains(out, "3 tickets ok, 1 warning(s)") {
		t.Fatalf("check in a reopened change = %d:\n%s", code, out)
	}
	if code := runGate(root, []string{"breakdown", "approved"}); code != 0 {
		t.Fatal("re-approve breakdown")
	}
	code, out = checkOutput(t, root)
	if code != 0 || strings.Contains(out, "!") || !strings.Contains(out, "3 tickets ok\n") || strings.Contains(out, "warning") {
		t.Fatalf("check after re-approval = %d:\n%s", code, out)
	}
}

// T-0031 AC1, AC2: a title cut just after a "-" gives a valid name, and the
// next ADR gets the next number.
func TestAdrNewLongTitleTwice(t *testing.T) {
	root := workspace(t)
	title := strings.Repeat("a", 39) + " then more words"
	for i := 0; i < 2; i++ {
		if code, _ := captureStdout(t, func() int { return runAdr(root, []string{"new", "--title", title}) }); code != 0 {
			t.Fatalf("adr new %d = %d", i+1, code)
		}
	}
	names, err := filepath.Glob(filepath.Join(root, "04_architecture", "adr", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"0001-" + strings.Repeat("a", 39) + ".md", "0002-" + strings.Repeat("a", 39) + ".md"}
	if len(names) != 2 || filepath.Base(names[0]) != want[0] || filepath.Base(names[1]) != want[1] {
		t.Fatalf("names = %v, want %v", names, want)
	}
}
