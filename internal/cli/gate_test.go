package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/zumen/internal/state"
)

// Artifact checks on `zumen gate` (S-0026). The change is 1-auth-flow.

const (
	archFile = "specs/1-auth-flow/architecture.md"
	specFile = "specs/1-auth-flow/functional-spec.md"
)

func writeChangeFile(t *testing.T, root, name, text string) {
	t.Helper()
	p := filepath.Join(activeChangeDir(t, root), name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gateStderr(t *testing.T, root string, args ...string) (int, string) {
	t.Helper()
	return captureStderr(t, func() int { return runGate(root, args) })
}

func gates(t *testing.T, root string) map[string]string {
	t.Helper()
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	return st.Gates
}

// noDiagrams has two problems: no ## Diagrams, and a slot on line 5.
const noDiagrams = "# Architecture — Auth flow\n\n## Overview\n\nTalks to <component>.\n"

// AC1.
func TestGateArchitectureRefusedUnderBlock(t *testing.T) {
	root := workspace(t)
	writeChangeFile(t, root, "architecture.md", noDiagrams)
	approveThrough(t, root, "spec")
	code, stderr := gateStderr(t, root, "architecture", "approved")
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	for _, want := range []string{
		"zumen: architecture approved: 2 artifact problem(s):\n",
		"  " + archFile + ": no ## Diagrams section\n",
		"  " + archFile + ":5: unfilled template placeholder <component>\n",
		"(pass --force to override)\n",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	if g := gates(t, root)["architecture"]; g != "" {
		t.Fatalf("architecture gate = %q, want unchanged", g)
	}
	if strings.Contains(readLog(t, root), "architecture") {
		t.Fatal("a refused gate wrote to LOG.md")
	}
}

// AC2: --force approves and logs one override line per problem.
func TestGateArtifactForceLogsEachProblem(t *testing.T) {
	root := workspace(t)
	writeChangeFile(t, root, "architecture.md", noDiagrams)
	approveThrough(t, root, "spec")
	if code, stderr := gateStderr(t, root, "architecture", "approved", "--force"); code != 0 {
		t.Fatalf("code = %d: %s", code, stderr)
	}
	if g := gates(t, root)["architecture"]; g != "approved" {
		t.Fatalf("architecture gate = %q", g)
	}
	log := readLog(t, root)
	if n := strings.Count(log, "override:"); n != 2 {
		t.Fatalf("%d override lines, want 2:\n%s", n, log)
	}
	for _, want := range []string{
		"override: " + archFile + ": no ## Diagrams section",
		"override: " + archFile + ":5: unfilled template placeholder <component>",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("LOG.md lacks %q:\n%s", want, log)
		}
	}
}

// AC2: under prompt it warns and advances.
func TestGateArtifactPromptWarns(t *testing.T) {
	root := workspace(t)
	writeChangeFile(t, root, "architecture.md", noDiagrams)
	approveThrough(t, root, "spec")
	setGates(t, root, "prompt")
	code, stderr := gateStderr(t, root, "architecture", "approved")
	if code != 0 || !strings.Contains(stderr, "warning: zumen: architecture approved: 2 artifact problem(s):") {
		t.Fatalf("code = %d: %s", code, stderr)
	}
	if strings.Contains(stderr, "--force") {
		t.Fatalf("a warning suggests --force: %s", stderr)
	}
	if g := gates(t, root)["architecture"]; g != "approved" {
		t.Fatalf("architecture gate = %q", g)
	}
	if n := strings.Count(readLog(t, root), "override:"); n != 2 {
		t.Fatalf("%d override lines, want 2", n)
	}
}

// AC3, AC4.
func TestGateArchitectureNamesSubsectionAndLine(t *testing.T) {
	cases := []struct {
		name, arch, want string
	}{
		{"no Flow",
			strings.Replace(testArchitecture, "### Flow: sign in", "### Notes", 1),
			archFile + ": ## Diagrams has no ### Flow: <name>"},
		{"empty Flow block",
			strings.Replace(testArchitecture, "sequenceDiagram\n  user->>app: sign in\n", "", 1),
			archFile + ":19: ### Flow: sign in needs a non-empty mermaid block"},
		{"slot",
			strings.Replace(testArchitecture, "## Diagrams", "Uses <component>.\n\n## Diagrams", 1),
			archFile + ":3: unfilled template placeholder <component>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := workspace(t)
			writeChangeFile(t, root, "architecture.md", c.arch)
			approveThrough(t, root, "spec")
			code, stderr := gateStderr(t, root, "architecture", "approved")
			if code != 1 || !strings.Contains(stderr, "  "+c.want+"\n") {
				t.Fatalf("code = %d, want 1 naming %q:\n%s", code, c.want, stderr)
			}
		})
	}
}

// AC5.
func TestGateSpecRefused(t *testing.T) {
	for _, c := range []struct {
		name, spec, want string
	}{
		{"missing", "", specFile + ": doesn't exist"},
		{"template only",
			"# Functional spec — X\n\n## Functional requirements\n\n- R1. <capability, verifiable> — **must | should | could**\n",
			specFile + ": no R requirements under ## Functional requirements"},
		{"duplicate id",
			"## Functional requirements\n\n- R1. A. — **must**\n- R1. B. — **must**\n",
			specFile + ": line 4: R1 is already defined on line 3"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := workspace(t)
			if c.spec != "" {
				writeChangeFile(t, root, "functional-spec.md", c.spec)
			}
			if code := runGate(root, []string{"intake", "approved"}); code != 0 {
				t.Fatal("intake")
			}
			if code := runGate(root, []string{"research", "skipped", "--reason", "none"}); code != 0 {
				t.Fatal("research")
			}
			code, stderr := gateStderr(t, root, "spec", "approved")
			if code != 1 || !strings.Contains(stderr, "  "+c.want+"\n") {
				t.Fatalf("code = %d, want 1 naming %q:\n%s", code, c.want, stderr)
			}
			if g := gates(t, root)["spec"]; g == "approved" {
				t.Fatal("spec was approved")
			}
		})
	}
}

// AC6: no tickets, or a ticket that fails validation, blocks breakdown.
func TestGateBreakdownRefused(t *testing.T) {
	root := workspace(t)
	approveThrough(t, root, "architecture")
	code, stderr := gateStderr(t, root, "breakdown", "approved")
	if code != 1 || !strings.Contains(stderr, "  specs/1-auth-flow/tickets: no tickets\n") {
		t.Fatalf("no tickets: code = %d:\n%s", code, stderr)
	}

	ticketRepo(t, root)
	if code := runTickets(root, []string{"new", "story", "Orphan", "--parent", "F-0099", "--repo", "app"}); code != 0 {
		t.Fatal("new Orphan")
	}
	code, stderr = gateStderr(t, root, "breakdown", "approved")
	want := "  specs/1-auth-flow/tickets/S-0001.md: S-0001: parent F-0099 doesn't exist\n"
	if code != 1 || !strings.Contains(stderr, want) {
		t.Fatalf("invalid ticket: code = %d, want 1 naming the ticket:\n%s", code, stderr)
	}
	if g := gates(t, root)["breakdown"]; g == "approved" {
		t.Fatal("breakdown was approved")
	}
}

// AC6: a `tickets check` warning alone doesn't block breakdown.
func TestGateBreakdownWarningsDontBlock(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	if code := runTickets(root, []string{"new", "story", "Login", "--repo", "app"}); code != 0 {
		t.Fatal("new Login")
	}
	fillCriteria(t, root, "S-0001")
	approveThrough(t, root, "breakdown")
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0001"}) }); code != 0 {
		t.Fatalf("ready = %d: %s", code, out)
	}
	reopen(t, root, "breakdown")
	if code, out := checkOutput(t, root); code != 0 || !strings.Contains(out, "! S-0001: ready") {
		t.Fatalf("check = %d, want a warning:\n%s", code, out)
	}
	if code, stderr := gateStderr(t, root, "breakdown", "approved"); code != 0 || strings.Contains(stderr, "artifact") {
		t.Fatalf("re-approve = %d:\n%s", code, stderr)
	}
}

// AC7.
func TestGatePublishCompleteNeedsFrozenTickets(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	if code := runTickets(root, []string{"new", "story", "Login", "--repo", "app"}); code != 0 {
		t.Fatal("new Login")
	}
	fillCriteria(t, root, "S-0001")
	startPublish(t, root)
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0001"}) }); code != 0 {
		t.Fatalf("ready = %d: %s", code, out)
	}
	code, stderr := gateStderr(t, root, "publish", "complete")
	want := "  specs/1-auth-flow/tickets/S-0001.md: S-0001: ready but not frozen (no published_hash)\n"
	if code != 1 || !strings.Contains(stderr, want) {
		t.Fatalf("code = %d, want 1 naming S-0001:\n%s", code, stderr)
	}
	if code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1"); code != 0 {
		t.Fatalf("mark-published = %d: %s", code, out)
	}
	if code, stderr := gateStderr(t, root, "publish", "complete"); code != 0 {
		t.Fatalf("after publishing: code = %d:\n%s", code, stderr)
	}
}

// AC8: other statuses aren't checked, even with no artifacts at all.
func TestGateUncheckedStatuses(t *testing.T) {
	root := workspace(t)
	if code := runGate(root, []string{"intake", "approved"}); code != 0 {
		t.Fatal("intake")
	}
	if code := runGate(root, []string{"research", "skipped", "--reason", "none"}); code != 0 {
		t.Fatal("research")
	}
	for _, args := range [][]string{{"spec", "await-approval"}, {"spec", "draft"}} {
		if code, stderr := gateStderr(t, root, args...); code != 0 || strings.Contains(stderr, "artifact") {
			t.Fatalf("gate %v = %d:\n%s", args, code, stderr)
		}
	}

	root = workspace(t)
	approveThrough(t, root, "breakdown")
	for _, name := range []string{"functional-spec.md", "architecture.md"} {
		if err := os.Remove(filepath.Join(activeChangeDir(t, root), name)); err != nil {
			t.Fatal(err)
		}
	}
	if code, stderr := gateStderr(t, root, "publish", "in-progress"); code != 0 || strings.Contains(stderr, "artifact") {
		t.Fatalf("publish in-progress = %d:\n%s", code, stderr)
	}
}

// AC9: a complete workspace goes from intake to publish complete with no override.
func TestGateCompleteLoopUnderBlock(t *testing.T) {
	root := workspace(t)
	writeArtifacts(t, root)
	ticketRepo(t, root)
	if code := runTickets(root, []string{"new", "story", "Login", "--repo", "app"}); code != 0 {
		t.Fatal("new Login")
	}
	fillCriteria(t, root, "S-0001")
	for _, args := range [][]string{
		{"intake", "approved"},
		{"research", "skipped", "--reason", "none"},
		{"spec", "approved"},
		{"architecture", "approved"},
		{"breakdown", "approved"},
		{"publish", "in-progress"},
	} {
		if code, stderr := gateStderr(t, root, args...); code != 0 {
			t.Fatalf("gate %v = %d:\n%s", args, code, stderr)
		}
	}
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0001"}) }); code != 0 {
		t.Fatalf("ready = %d: %s", code, out)
	}
	if code, out := markPublished(t, root, "S-0001", "--external", "github=o/r#1"); code != 0 {
		t.Fatalf("mark-published = %d: %s", code, out)
	}
	if code, stderr := gateStderr(t, root, "publish", "complete"); code != 0 {
		t.Fatalf("publish complete = %d:\n%s", code, stderr)
	}
	if log := readLog(t, root); strings.Contains(log, "override") {
		t.Fatalf("a complete loop logged an override:\n%s", log)
	}
}

// AC10: an order violation and artifact problems are refused together.
func TestGateOrderAndArtifactProblemsTogether(t *testing.T) {
	root := workspace(t)
	code, stderr := gateStderr(t, root, "architecture", "approved")
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	want := `zumen: architecture requires intake to be approved (currently "draft")` + "\n" +
		"zumen: architecture approved: 1 artifact problem(s):\n" +
		"  " + archFile + ": doesn't exist\n" +
		"(pass --force to override)\n"
	if !strings.HasSuffix(stderr, want) {
		t.Fatalf("stderr:\n%s\nwant it to end with:\n%s", stderr, want)
	}
	if g := gates(t, root)["architecture"]; g != "" {
		t.Fatalf("architecture gate = %q", g)
	}
}

// A lone order violation keeps today's one-line refusal.
func TestGateOrderViolationAloneIsOneLine(t *testing.T) {
	root := workspace(t)
	writeArtifacts(t, root)
	_, stderr := gateStderr(t, root, "architecture", "approved")
	want := `zumen: architecture requires intake to be approved (currently "draft") (pass --force to override)` + "\n"
	if !strings.HasSuffix(stderr, want) {
		t.Fatalf("stderr = %q, want suffix %q", stderr, want)
	}
}

// Reopening breakdown to revise a ready ticket doesn't deadlock: the stale
// readiness mark doesn't block re-approval, tickets check still reports it,
// and re-readying after approval clears it (review finding 1).
func TestGateBreakdownReopenReviseReready(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	if code := runTickets(root, []string{"new", "story", "Login", "--repo", "app"}); code != 0 {
		t.Fatal("new Login")
	}
	fillCriteria(t, root, "S-0001")
	approveThrough(t, root, "breakdown")
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0001"}) }); code != 0 {
		t.Fatalf("ready = %d: %s", code, out)
	}
	reopen(t, root, "breakdown")
	path := ticketPath(root, "S-0001")
	if err := os.WriteFile(path, []byte(readFile(t, path)+"\nRevised during the reopened breakdown.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := checkOutput(t, root); code != 1 || !strings.Contains(out, "✗ S-0001: body changed since readied") {
		t.Fatalf("check before re-approval = %d:\n%s", code, out)
	}
	if code, stderr := gateStderr(t, root, "breakdown", "approved"); code != 0 || strings.Contains(stderr, "artifact") {
		t.Fatalf("re-approve = %d:\n%s", code, stderr)
	}
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "S-0001"}) }); code != 0 {
		t.Fatalf("re-ready = %d: %s", code, out)
	}
	if code, out := checkOutput(t, root); code != 0 || strings.Contains(out, "✗") {
		t.Fatalf("check after re-ready = %d:\n%s", code, out)
	}
	if strings.Contains(readLog(t, root), "override") {
		t.Fatal("the flow needed an override")
	}
}
