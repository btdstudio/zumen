package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Overview auto-regeneration (S-0029). The change is 1-auth-flow.

const authReadme = "specs/1-auth-flow/README.md"

// refreshWorkspace is a workspace with change 1-auth-flow, the app repo and
// valid artifacts. Under git, the repo is initialised first.
func refreshWorkspace(t *testing.T, settingsJSON string, withGit bool) string {
	t.Helper()
	root := vcsWorkspace(t, settingsJSON)
	if withGit {
		gitRepo(t, root)
	}
	if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
		t.Fatal("change new")
	}
	ticketRepo(t, root)
	writeArtifacts(t, root)
	return root
}

func head(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
}

// inHead fails unless the last commit includes path and nothing under specs/
// is left uncommitted.
func inHead(t *testing.T, root, step, path string) {
	t.Helper()
	if files := git(t, root, "show", "--name-only", "--format=", "HEAD"); !strings.Contains(files, path+"\n") {
		t.Fatalf("%s: HEAD lacks %s:\n%s", step, path, files)
	}
	if status := git(t, root, "status", "--porcelain", "--", "specs"); strings.TrimSpace(status) != "" {
		t.Fatalf("%s: left uncommitted:\n%s", step, status)
	}
}

// current fails unless `zumen overview --check` passes.
func current(t *testing.T, root, step string) {
	t.Helper()
	if code, out, _ := overviewRun(t, root, "1-auth-flow", "--check"); code != 0 {
		t.Fatalf("%s: --check = %d, %q", step, code, out)
	}
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

// AC1, AC2, AC7: under auto and git, each command regenerates the README, and
// all but tickets new commit it.
func TestOverviewRegeneratedIntoCommits(t *testing.T) {
	root := refreshWorkspace(t, "", true)
	steps := []struct {
		name string
		run  func() int
	}{
		{"gate intake", func() int { return runGate(root, []string{"intake", "approved"}) }},
		{"skip research", func() int { return runGate(root, []string{"research", "skipped", "--reason", "none"}) }},
		{"gate spec", func() int { return runGate(root, []string{"spec", "approved"}) }},
		{"gate architecture", func() int { return runGate(root, []string{"architecture", "approved"}) }},
		{"reopen architecture", func() int { return runReopen(root, []string{"architecture", "--reason", "rework"}) }},
		{"re-gate architecture", func() int { return runGate(root, []string{"architecture", "approved"}) }},
	}
	for _, s := range steps {
		if code := s.run(); code != 0 {
			t.Fatalf("%s = %d", s.name, code)
		}
		inHead(t, root, s.name, authReadme)
		current(t, root, s.name)
	}

	// tickets new regenerates but doesn't record.
	before, logBefore := head(t, root), readLog(t, root)
	if code := runTickets(root, []string{"new", "story", "Login", "--repo", "app", "--requirements", "R1"}); code != 0 {
		t.Fatal("tickets new")
	}
	if head(t, root) != before || readLog(t, root) != logBefore {
		t.Fatal("tickets new committed or logged")
	}
	if status := git(t, root, "status", "--porcelain", "--", authReadme); !strings.HasPrefix(status, " M ") {
		t.Fatalf("tickets new left the README %q, want modified", status)
	}
	if !strings.Contains(readFile(t, filepath.Join(root, authReadme)), "[**S-0001**](tickets/S-0001.md) Login") {
		t.Fatal("tickets new didn't regenerate the README")
	}
	current(t, root, "tickets new")

	fillCriteria(t, root, "S-0001")
	for _, s := range []struct {
		name string
		run  func() int
	}{
		{"gate breakdown", func() int { return runGate(root, []string{"breakdown", "approved"}) }},
		{"tickets ready", func() int { return runTickets(root, []string{"ready", "S-0001"}) }},
		{"publish in-progress", func() int { return runGate(root, []string{"publish", "in-progress"}) }},
		{"mark-published", func() int {
			return runTickets(root, []string{"mark-published", "S-0001", "--external", "github=o/app#1"})
		}},
	} {
		var code int
		captureStdout(t, func() int { code = s.run(); return code })
		if code != 0 {
			t.Fatalf("%s = %d", s.name, code)
		}
		inHead(t, root, s.name, authReadme)
		current(t, root, s.name)
	}

	// unfreeze commits the README when it changes, and leaves it current either way.
	if code := runTickets(root, []string{"unfreeze", "S-0001", "--reason", "rework"}); code != 0 {
		t.Fatal("unfreeze")
	}
	if status := git(t, root, "status", "--porcelain", "--", "specs"); strings.TrimSpace(status) != "" {
		t.Fatalf("unfreeze left uncommitted:\n%s", status)
	}
	current(t, root, "unfreeze")
}

// AC3: under manual none of the six commands touches the README.
func TestOverviewManualLeavesReadmeAlone(t *testing.T) {
	root := refreshWorkspace(t, `{"vcs": "none", "overview": "manual"}`, false)
	if code := runTickets(root, []string{"new", "story", "Login", "--repo", "app", "--requirements", "R1"}); code != 0 {
		t.Fatal("tickets new")
	}
	fillCriteria(t, root, "S-0001")
	approveThrough(t, root, "breakdown")
	if code := runReopen(root, []string{"breakdown", "--reason", "rework"}); code != 0 {
		t.Fatal("reopen")
	}
	for _, args := range [][]string{
		{"gate", "breakdown", "approved"},
		{"tickets", "ready", "S-0001"},
		{"gate", "publish", "in-progress"},
		{"tickets", "mark-published", "S-0001", "--external", "github=o/app#1"},
		{"tickets", "unfreeze", "S-0001", "--reason", "rework"},
	} {
		var code int
		captureStdout(t, func() int {
			if args[0] == "gate" {
				code = runGate(root, args[1:])
			} else {
				code = runTickets(root, args[1:])
			}
			return code
		})
		if code != 0 {
			t.Fatalf("%v = %d", args, code)
		}
	}
	if exists(root, authReadme) {
		t.Fatal("manual wrote the README")
	}
}

// AC4: a malformed README is a warning; the commands succeed and it's untouched.
func TestOverviewMalformedIsAWarning(t *testing.T) {
	root := refreshWorkspace(t, "", true)
	if code := runTickets(root, []string{"new", "story", "Login", "--repo", "app", "--requirements", "R1"}); code != 0 {
		t.Fatal("tickets new")
	}
	fillCriteria(t, root, "S-0001")
	approveThrough(t, root, "breakdown")
	bad := "<!-- zumen:narrative -->\n<!-- /zumen:narrative -->\n<!-- zumen:generated -->\n<!-- zumen:generated -->\n<!-- /zumen:generated -->\n"
	path := filepath.Join(root, authReadme)
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	warning := "warning: zumen: " + authReadme + ": duplicated marker <!-- zumen:generated --> (lines 3 and 4)"

	code, stderr := captureStderr(t, func() int { return runGate(root, []string{"publish", "in-progress"}) })
	if code != 0 || !strings.Contains(stderr, warning) || gates(t, root)["publish"] != "in-progress" {
		t.Fatalf("gate = %d, %q", code, stderr)
	}
	if readFile(t, path) != bad {
		t.Fatal("the gate changed the README")
	}

	code, stderr = captureStderr(t, func() int {
		c, _ := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0001"}) })
		return c
	})
	if code != 0 || !strings.Contains(stderr, warning) {
		t.Fatalf("ready = %d, %q", code, stderr)
	}
	if readFile(t, path) != bad {
		t.Fatal("ready changed the README")
	}
	if files := git(t, root, "show", "--name-only", "--format=", "HEAD"); strings.Contains(files, authReadme) || !strings.Contains(files, "S-0001.md") {
		t.Fatalf("ready's commit:\n%s", files)
	}
}

// AC5: an invalid overview value fails every command that loads settings.
func TestOverviewInvalidSetting(t *testing.T) {
	root := refreshWorkspace(t, `{"vcs": "none"}`, false)
	if err := os.WriteFile(filepath.Join(root, ".zumen", "settings.json"), []byte(`{"vcs": "none", "overview": "sometimes"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	want := `zumen: invalid overview "sometimes" (expected auto|manual)`
	for name, run := range map[string]func() int{
		"status": func() int { return runStatus(root) },
		"gate":   func() int { return runGate(root, []string{"intake", "approved"}) },
	} {
		if code, stderr := captureStderr(t, run); code == 0 || !strings.Contains(stderr, want) {
			t.Fatalf("%s = %d, %q", name, code, stderr)
		}
	}
}

// AC6: tickets ready across two changes regenerates and commits both READMEs.
func TestOverviewReadyAcrossChanges(t *testing.T) {
	root := refreshWorkspace(t, "", true)
	if code := runTickets(root, []string{"new", "story", "Login", "--repo", "app", "--requirements", "R1"}); code != 0 {
		t.Fatal("tickets new 1")
	}
	fillCriteria(t, root, "S-0001")
	approveThrough(t, root, "breakdown")
	if code := runChange(root, []string{"new", "Other work"}); code != 0 {
		t.Fatal("change new 2")
	}
	writeArtifacts(t, root)
	if code := runTickets(root, []string{"new", "story", "Logout", "--repo", "app", "--requirements", "R1"}); code != 0 {
		t.Fatal("tickets new 2")
	}
	other := filepath.Join(root, "specs", "2-other-work", "tickets", "S-0002.md")
	text := strings.Replace(readFile(t, other), "- [ ] AC1: Given … When … Then …", "- [ ] AC1: Given a user, When they log out, Then they're signed out", 1)
	if err := os.WriteFile(other, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	approveThrough(t, root, "breakdown")
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0001", "S-0002"}) }); code != 0 {
		t.Fatalf("ready = %d: %s", code, out)
	}
	inHead(t, root, "ready", authReadme)
	inHead(t, root, "ready", "specs/2-other-work/README.md")
}

// R1: zumen settings shows the effective overview value.
func TestSettingsShowsOverview(t *testing.T) {
	for _, tc := range []struct{ settings, want string }{
		{`{"vcs": "none"}`, "overview:    auto\n"},
		{`{"vcs": "none", "overview": "manual"}`, "overview:    manual\n"},
	} {
		root := vcsWorkspace(t, tc.settings)
		if code, out := captureStdout(t, func() int { return runSettings(root) }); code != 0 || !strings.Contains(out, tc.want) {
			t.Fatalf("%s: settings = %d, %q", tc.settings, code, out)
		}
	}
}

// A REPOS.json error names the README once, without a doubled "zumen: ".
func TestOverviewBadReposWarning(t *testing.T) {
	root := refreshWorkspace(t, `{"vcs": "none"}`, false)
	if err := os.WriteFile(filepath.Join(root, "REPOS.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stderr := captureStderr(t, func() int { return runGate(root, []string{"intake", "approved"}) })
	if code != 0 || !strings.Contains(stderr, "warning: zumen: "+authReadme+": cannot parse ") || strings.Count(stderr, "zumen: ") != 1 {
		t.Fatalf("gate = %d, %q", code, stderr)
	}
	code, _, stderr = overviewRun(t, root, "1-auth-flow", "--check")
	if code == 0 || strings.Count(stderr, "zumen: ") != 1 {
		t.Fatalf("overview = %d, %q", code, stderr)
	}
}
