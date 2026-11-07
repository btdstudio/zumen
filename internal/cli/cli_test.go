package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/zumen/internal/state"
)

// workspace returns an isolated root with one greenfield change at intake.
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
	if code := runGate(root, []string{"design", "approved"}); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Gates["design"] != "" || st.Phase != "intake" {
		t.Fatalf("refused gate changed state: %+v", st)
	}
}

func TestGateFollowsLoopInOrder(t *testing.T) {
	root := workspace(t)
	for _, phase := range []string{"intake", "requirements", "design", "plan"} {
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
	if code := runGate(root, []string{"design", "approved", "--force"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if log := readLog(t, root); !strings.Contains(log, "override: zumen: design requires intake") {
		t.Fatalf("override not logged: %s", log)
	}
}

func TestGateNonBlockSettingWarnsAndProceeds(t *testing.T) {
	root := workspace(t)
	setGates(t, root, "auto")
	if code := runGate(root, []string{"design", "approved"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if log := readLog(t, root); !strings.Contains(log, "override:") {
		t.Fatalf("override not logged: %s", log)
	}
}

func TestGateRejectsBadNamesEvenWithForce(t *testing.T) {
	root := workspace(t)
	before := readLog(t, root)
	if code := runGate(root, []string{"design", "nope", "--force"}); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if log := readLog(t, root); log != before {
		t.Fatalf("log = %q", log)
	}
}

func TestExportRequiresApprovedPlan(t *testing.T) {
	root := workspace(t)
	spec := filepath.Join(state.SpecsDir(root), "1-auth-flow", "tasks.json")
	if err := os.WriteFile(spec, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	exported := filepath.Join(target, "01_planning", "specs", "1-auth-flow", "tasks.json")

	if code := runExport(root, []string{"--to", target}); code != 1 {
		t.Fatalf("unapproved export = %d, want 1", code)
	}
	if _, err := os.Stat(exported); err == nil {
		t.Fatal("refused export wrote files")
	}
	if code := runExport(root, []string{"--to", target, "--force"}); code != 0 {
		t.Fatalf("forced export = %d", code)
	}
	if _, err := os.Stat(exported); err != nil {
		t.Fatal(err)
	}
	if log := readLog(t, root); !strings.Contains(log, "override: export:") {
		t.Fatalf("override not logged: %s", log)
	}

	// Approved, but the destination now exists: still refused without --force.
	for _, phase := range []string{"intake", "requirements", "design", "plan"} {
		if code := runGate(root, []string{phase, "approved"}); code != 0 {
			t.Fatalf("gate %s = %d", phase, code)
		}
	}
	if code := runExport(root, []string{"--to", target}); code != 1 {
		t.Fatalf("overwriting export = %d, want 1", code)
	}
	if code := runExport(root, []string{"--to", t.TempDir()}); code != 0 {
		t.Fatalf("approved export = %d", code)
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
	if code := runGate(root, []string{"requirements", "draft"}); code != 1 {
		t.Fatalf("requirements before explore = %d, want 1", code)
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
	if code := runGate(root, []string{"requirements", "draft"}); code != 0 {
		t.Fatalf("requirements after skip = %d", code)
	}
}

func TestReopenClearsLaterApprovals(t *testing.T) {
	root := workspace(t)
	for _, phase := range []string{"intake", "requirements", "design"} {
		if code := runGate(root, []string{phase, "approved"}); code != 0 {
			t.Fatalf("gate %s = %d", phase, code)
		}
	}
	if code := runReopen(root, []string{"requirements"}); code != 1 {
		t.Fatalf("reopen without reason = %d, want 1", code)
	}
	if code := runReopen(root, []string{"requirements", "--reason", "scope changed"}); code != 0 {
		t.Fatalf("reopen = %d", code)
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Gates["requirements"] != "draft" || st.Gates["design"] != "" || st.Gates["intake"] != "approved" || st.Gates["explore"] != "skipped" {
		t.Fatalf("gates = %+v", st.Gates)
	}
	if log := readLog(t, root); !strings.Contains(log, "reopened requirements (scope changed)") {
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
