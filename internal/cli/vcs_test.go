package cli

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/btdstudio/zumen/internal/state"
)

const localHint = `set "vcs": "none" if this workspace is meant to be local`

// vcsWorkspace returns an isolated root whose project settings hold settingsJSON
// (none written when empty). No change exists yet.
func vcsWorkspace(t *testing.T, settingsJSON string) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if settingsJSON != "" {
		dir := state.Workspace(root)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func captureStderr(t *testing.T, fn func() int) (int, string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	code := fn()
	os.Stderr = orig
	w.Close()
	out, _ := io.ReadAll(r)
	return code, string(out)
}

// gitRepo makes root a git repo with a local identity, skipping without git.
func gitRepo(t *testing.T, root string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "test"},
		{"config", "user.email", "test@example.com"},
		{"config", "commit.gpgsign", "false"},
		// No background gc or maintenance: it can still be writing to .git when
		// the test's temp dir is removed.
		{"config", "gc.auto", "0"},
		{"config", "maintenance.auto", "false"},
	} {
		git(t, root, args...)
	}
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// AC1: a local workspace records transitions without git, warnings or .gitkeep.
func TestVCSNoneTransitionsStaySilent(t *testing.T) {
	root := vcsWorkspace(t, `{"vcs": "none"}`)
	steps := []func() int{
		func() int { return runChange(root, []string{"new", "Auth flow"}) },
		func() int { return runGate(root, []string{"intake", "approved"}) },
		func() int { return runReopen(root, []string{"intake", "--reason", "rethink"}) },
	}
	for i, step := range steps {
		code, stderr := captureStderr(t, step)
		if code != 0 {
			t.Fatalf("step %d = %d: %s", i, code, stderr)
		}
		if strings.Contains(stderr, "warning:") {
			t.Fatalf("step %d warned: %s", i, stderr)
		}
	}
	log := readLog(t, root)
	for _, want := range []string{"1-auth-flow: created", "intake → approved", "reopened intake (rethink)"} {
		if !strings.Contains(log, want) {
			t.Fatalf("LOG.md lacks %q:\n%s", want, log)
		}
	}
	if _, err := os.Stat(filepath.Join(state.ChangeDir(root, "1-auth-flow"), ".gitkeep")); !os.IsNotExist(err) {
		t.Fatalf(".gitkeep present in a local change folder (err = %v)", err)
	}
}

// AC2: vcs: none inside a git repo still makes no commit.
func TestVCSNoneMakesNoCommitInARepo(t *testing.T) {
	root := vcsWorkspace(t, `{"vcs": "none"}`)
	gitRepo(t, root)
	if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	if code := runGate(root, []string{"intake", "approved"}); code != 0 {
		t.Fatalf("gate = %d", code)
	}
	if out := git(t, root, "rev-list", "--all"); strings.TrimSpace(out) != "" {
		t.Fatalf("commits were made: %s", out)
	}
	if out := git(t, root, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "" {
		t.Fatalf("files were staged: %s", out)
	}
}

// AC3: the default outside a repo warns, with the hint to declare the workspace local.
func TestVCSDefaultOutsideRepoHints(t *testing.T) {
	root := vcsWorkspace(t, "")
	if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	code, stderr := captureStderr(t, func() int { return runGate(root, []string{"intake", "approved"}) })
	if code != 0 {
		t.Fatalf("gate = %d: %s", code, stderr)
	}
	if !strings.Contains(stderr, "not in a git repo") || !strings.Contains(stderr, localHint) {
		t.Fatalf("stderr = %q, want the warning with its hint", stderr)
	}
}

// AC4: the default inside a repo commits STATE.json and LOG.md as before.
func TestVCSDefaultInRepoCommits(t *testing.T) {
	root := vcsWorkspace(t, "")
	gitRepo(t, root)
	if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	if code := runGate(root, []string{"intake", "approved"}); code != 0 {
		t.Fatalf("gate = %d", code)
	}
	subjects := git(t, root, "log", "--format=%s")
	if !strings.Contains(subjects, "intake → approved") {
		t.Fatalf("gate not committed; log:\n%s", subjects)
	}
	files := git(t, root, "ls-tree", "-r", "--name-only", "HEAD")
	for _, want := range []string{"STATE.json", "LOG.md"} {
		if !strings.Contains(files, want) {
			t.Fatalf("HEAD lacks %s:\n%s", want, files)
		}
	}
}

// AC10: status shows the resolved vcs, with or without an active change.
func TestStatusShowsVCS(t *testing.T) {
	for _, c := range []struct{ settings, want string }{
		{"", "git"},
		{`{"vcs": "git"}`, "git"},
		{`{"vcs": "none"}`, "none"},
	} {
		root := vcsWorkspace(t, c.settings)
		want := "vcs:      " + c.want + "\n"
		code, out := captureStdout(t, func() int { return runStatus(root) })
		if code != 0 || !strings.Contains(out, "no active change\n"+want) {
			t.Fatalf("settings %q, no change: code %d, out %q", c.settings, code, out)
		}
		if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
			t.Fatalf("change new = %d", code)
		}
		code, out = captureStdout(t, func() int { return runStatus(root) })
		if code != 0 || !strings.Contains(out, "gates:    ") || !strings.HasSuffix(out, want) {
			t.Fatalf("settings %q, active change: code %d, out %q", c.settings, code, out)
		}
	}
}

// AC11: an invalid vcs makes status fail, naming the key and the value.
func TestStatusRejectsInvalidVCS(t *testing.T) {
	root := vcsWorkspace(t, `{"vcs": "svn"}`)
	code, stderr := captureStderr(t, func() int { return runStatus(root) })
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "vcs") || !strings.Contains(stderr, "svn") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// fakeGit puts a git on PATH (alone) that records every call in a marker file and
// fails, so a test can prove zumen never ran git. It returns the marker path.
func fakeGit(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake")
	}
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "git-called")
	script := "#!/bin/sh\necho \"$@\" >> '" + marker + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return marker
}

// R2 (QA): under vcs: none every command that records a transition — change new,
// gate, reopen, change close, and the ticket commands that save through Record —
// runs no git at all and prints no warning.
func TestVCSNoneRunsNoGitForAnyRecordedCommand(t *testing.T) {
	root := vcsWorkspace(t, `{"vcs": "none"}`)
	ticketRepo(t, root) // before git is faked: the repo registry is out of scope
	marker := fakeGit(t)
	steps := []struct {
		name string
		run  func() int
	}{
		{"change new", func() int { return runChange(root, []string{"new", "Auth flow"}) }},
		{"gate", func() int { return runGate(root, []string{"intake", "approved"}) }},
		{"reopen", func() int { return runReopen(root, []string{"intake", "--reason", "rethink"}) }},
		{"tickets new", func() int { return runTickets(root, []string{"new", "story", "A", "--repo", "app"}) }},
		{"gates through publish", func() int {
			fillCriteria(t, root, "S-0001")
			startPublish(t, root)
			return 0
		}},
		{"tickets ready", func() int { return runTickets(root, []string{"ready", "S-0001"}) }},
		{"tickets mark-published", func() int {
			return runTickets(root, []string{"mark-published", "S-0001", "--external", "github=org/app#7"})
		}},
		{"change close", func() int { return runChange(root, []string{"close", "1-auth-flow", "--reason", "done"}) }},
	}
	for _, step := range steps {
		code, stderr := captureStderr(t, step.run)
		if code != 0 {
			t.Fatalf("%s = %d: %s", step.name, code, stderr)
		}
		if strings.Contains(stderr, "warning:") {
			t.Fatalf("%s warned: %s", step.name, stderr)
		}
		if calls, err := os.ReadFile(marker); err == nil {
			t.Fatalf("%s ran git: %s", step.name, calls)
		}
	}
	log := readLog(t, root)
	for _, want := range []string{"S-0001 published to github (org/app#7)", "closed (done)"} {
		if !strings.Contains(log, want) {
			t.Fatalf("LOG.md lacks %q:\n%s", want, log)
		}
	}
}

// tickets ready records every ticket in one commit, with each ticket file in it.
func TestTicketsReadyCommitsOnce(t *testing.T) {
	root := vcsWorkspace(t, "")
	gitRepo(t, root)
	if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	ticketRepo(t, root)
	for _, title := range []string{"Login", "Logout"} {
		if code := runTickets(root, []string{"new", "story", title, "--repo", "app"}); code != 0 {
			t.Fatalf("new %s", title)
		}
	}
	fillCriteria(t, root, "S-0001")
	fillCriteria(t, root, "S-0002")
	approveThrough(t, root, "breakdown")
	before := strings.Count(git(t, root, "log", "--format=%s"), "\n")
	if code := runTickets(root, []string{"ready", "--all"}); code != 0 {
		t.Fatal("ready --all")
	}
	if after := strings.Count(git(t, root, "log", "--format=%s"), "\n"); after != before+1 {
		t.Fatalf("commits: %d → %d, want exactly one more", before, after)
	}
	files := git(t, root, "show", "--name-only", "--format=", "HEAD")
	for _, want := range []string{"LOG.md", "specs/1-auth-flow/tickets/S-0001.md", "specs/1-auth-flow/tickets/S-0002.md"} {
		if !strings.Contains(files, want) {
			t.Fatalf("HEAD lacks %s:\n%s", want, files)
		}
	}
	if status := git(t, root, "status", "--porcelain", "--", "specs"); strings.TrimSpace(status) != "" {
		t.Fatalf("ticket files left uncommitted:\n%s", status)
	}
}

// A forced mark-published logs its override and the publish in one commit.
func TestTicketsMarkPublishedCommitsOnce(t *testing.T) {
	root := vcsWorkspace(t, "")
	gitRepo(t, root)
	if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	ticketRepo(t, root)
	// The draft exists before breakdown, so breakdown has a ticket to approve.
	if code := runTickets(root, []string{"new", "story", "Login", "--repo", "app"}); code != 0 {
		t.Fatal("new Login")
	}
	fillCriteria(t, root, "S-0001")
	approveThrough(t, root, "breakdown")
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0001"}) }); code != 0 {
		t.Fatalf("ready = %d: %s", code, out)
	}
	before := strings.Count(git(t, root, "log", "--format=%s"), "\n")
	if code := runTickets(root, []string{"mark-published", "S-0001", "--external", "github=o/r#1", "--force"}); code != 0 {
		t.Fatal("mark-published --force")
	}
	if after := strings.Count(git(t, root, "log", "--format=%s"), "\n"); after != before+1 {
		t.Fatalf("commits: %d → %d, want exactly one more", before, after)
	}
	diff := git(t, root, "show", "--format=", "HEAD", "--", "LOG.md")
	if !strings.Contains(diff, "override:") || !strings.Contains(diff, "S-0001 published to github") {
		t.Fatalf("HEAD's LOG.md change lacks a line:\n%s", diff)
	}
	if status := git(t, root, "status", "--porcelain", "--", "specs", "LOG.md"); strings.TrimSpace(status) != "" {
		t.Fatalf("left uncommitted:\n%s", status)
	}
}
