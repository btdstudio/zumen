package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Requirement and ADR checks in tickets check, and tickets new --requirements
// (S-0027). The change is 1-auth-flow.

const citeSpec = "# Functional spec — Auth flow\n\n## Functional requirements\n\n" +
	"- R1. Sign in. — **must**\n- R2. Sign out. — **must**\n- R3. Remember me. — **should**\n- N1. Fast. — **must**\n"

// citeWorkspace has citeSpec, a valid architecture, the app repo, and no tickets.
func citeWorkspace(t *testing.T) string {
	t.Helper()
	root := workspace(t)
	ticketRepo(t, root)
	writeChangeFile(t, root, "functional-spec.md", citeSpec)
	writeArtifacts(t, root)
	return root
}

func newTicket(t *testing.T, root string, args ...string) {
	t.Helper()
	if code, out := captureStderr(t, func() int { return runTickets(root, append([]string{"new"}, args...)) }); code != 0 {
		t.Fatalf("tickets new %v = %d: %s", args, code, out)
	}
}

// editTicket replaces old with new in a ticket file.
func editTicket(t *testing.T, root, id, old, new string) {
	t.Helper()
	p := ticketPath(root, id)
	text := readFile(t, p)
	if !strings.Contains(text, old) {
		t.Fatalf("%s lacks %q:\n%s", id, old, text)
	}
	if err := os.WriteFile(p, []byte(strings.Replace(text, old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wantCheck(t *testing.T, root string, code int, want ...string) string {
	t.Helper()
	got, out := checkOutput(t, root)
	if got != code {
		t.Fatalf("check = %d, want %d:\n%s", got, code, out)
	}
	for _, w := range want {
		if !strings.Contains(out, w+"\n") {
			t.Fatalf("check output lacks %q:\n%s", w, out)
		}
	}
	return out
}

// AC1.
func TestTicketsCheckUnknownCitation(t *testing.T) {
	root := citeWorkspace(t)
	newTicket(t, root, "story", "Login", "--repo", "app", "--requirements", "R1,R2")
	editTicket(t, root, "S-0001", "requirements: [R1, R2]", "requirements: [R1, R2, R99]")
	wantCheck(t, root, 1, "✗ S-0001: cites R99, which isn't in 1-auth-flow's spec")
}

// AC2.
func TestTicketsCheckCoverage(t *testing.T) {
	root := citeWorkspace(t)
	if out := wantCheck(t, root, 0, "0 tickets ok"); strings.Contains(out, "must requirement") {
		t.Fatalf("a change with no tickets warned about coverage:\n%s", out)
	}
	newTicket(t, root, "story", "Login", "--repo", "app", "--requirements", "R1")
	out := wantCheck(t, root, 0,
		"! R2: must requirement in 1-auth-flow isn't cited by any ticket",
		"1 tickets ok, 1 warning(s)")
	if strings.Contains(out, "! N1") || strings.Contains(out, "! R3") {
		t.Fatalf("an N or should requirement warned:\n%s", out)
	}
}

// AC3.
func TestTicketsCheckReadyWithoutRequirements(t *testing.T) {
	root := citeWorkspace(t)
	newTicket(t, root, "epic", "Accounts")
	newTicket(t, root, "story", "Login", "--repo", "app", "--parent", "E-0001", "--requirements", "R1,R2")
	newTicket(t, root, "story", "Logout", "--repo", "app", "--parent", "E-0001")
	newTicket(t, root, "story", "Draft", "--repo", "app", "--parent", "E-0001")
	for _, id := range []string{"E-0001", "S-0002", "S-0003"} {
		fillCriteria(t, root, id)
	}
	approveThrough(t, root, "breakdown")
	if code, out := captureStdout(t, func() int { return runTickets(root, []string{"ready", "E-0001", "S-0002", "S-0003"}) }); code != 0 {
		t.Fatalf("ready = %d: %s", code, out)
	}
	out := wantCheck(t, root, 0, "! S-0003: ready story with no requirements")
	if strings.Contains(out, "E-0001: ready") || strings.Contains(out, "S-0004: ready") {
		t.Fatalf("an epic or a draft warned:\n%s", out)
	}
}

// AC4.
func TestTicketsCheckADRCitations(t *testing.T) {
	root := citeWorkspace(t)
	adrDir := filepath.Join(root, "04_architecture", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "0001-first.md"), []byte("# 0001 — First\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	newTicket(t, root, "story", "Login", "--repo", "app", "--requirements", "R1,R2")
	editTicket(t, root, "S-0001", "adrs: []", "adrs: [ADR-0001, ADR-0009, \"0001\"]")
	out := wantCheck(t, root, 0,
		"! S-0001: cites ADR-0009, but there's no 0009-*.md ADR file",
		`! S-0001: adrs entry "0001" isn't of the form ADR-<nnnn>`,
		"1 tickets ok, 2 warning(s)")
	if strings.Contains(out, "cites ADR-0001") {
		t.Fatalf("a resolved citation warned:\n%s", out)
	}
}

// AC5.
func TestTicketsNewRequirements(t *testing.T) {
	root := citeWorkspace(t)
	newTicket(t, root, "story", "Login", "--repo", "app", "--requirements", " R2 , ,R1,R2")
	if text := readFile(t, ticketPath(root, "S-0001")); !strings.Contains(text, "requirements: [R2, R1]\n") {
		t.Fatalf("S-0001:\n%s", text)
	}
	for _, c := range []struct{ list, want string }{
		{"R1,R9", "R9 isn't in 1-auth-flow's spec"},
		{"foo,R7", "foo isn't a requirement id (R<n> or N<n>); R7 isn't in 1-auth-flow's spec"},
		{"xR1,R1x,r1", "xR1 isn't a requirement id (R<n> or N<n>); R1x isn't a requirement id (R<n> or N<n>); r1 isn't a requirement id (R<n> or N<n>)"},
	} {
		code, stderr := captureStderr(t, func() int {
			return runTickets(root, []string{"new", "story", "Bad", "--repo", "app", "--requirements", c.list})
		})
		if code != 1 || !strings.Contains(stderr, "zumen tickets new: "+c.want) {
			t.Fatalf("--requirements %s = %d: %s", c.list, code, stderr)
		}
	}
	if _, err := os.Stat(ticketPath(root, "S-0002")); err == nil {
		t.Fatal("a refused tickets new wrote a file")
	}
	newTicket(t, root, "story", "Next", "--repo", "app", "--requirements", "N1")
	if text := readFile(t, ticketPath(root, "S-0002")); !strings.Contains(text, "requirements: [N1]\n") {
		t.Fatalf("the next ticket didn't get S-0002:\n%s", text)
	}
}

func TestTicketsNewRequirementsWithoutSpec(t *testing.T) {
	root := workspace(t)
	ticketRepo(t, root)
	code, stderr := captureStderr(t, func() int {
		return runTickets(root, []string{"new", "story", "Login", "--repo", "app", "--requirements", "R1"})
	})
	if code != 1 || !strings.Contains(stderr, "R1 can't be checked: 1-auth-flow has no spec") {
		t.Fatalf("code = %d: %s", code, stderr)
	}
}

// AC6.
func TestTicketsCheckSpecFindings(t *testing.T) {
	root := workspace(t)
	writeChangeFile(t, root, "functional-spec.md",
		"## Functional requirements\n\n- R2. A. — **should**\n- R2. B. — **should**\n- R4. C.\n")
	wantCheck(t, root, 1,
		"✗ R2: spec of 1-auth-flow: line 4: R2 is already defined on line 3",
		"! R4: spec of 1-auth-flow: line 5: R4 has no priority (expected — **must**, — **should** or — **could**)")
}

// AC7.
func TestGateBreakdownRefusesUnknownCitation(t *testing.T) {
	root := citeWorkspace(t)
	newTicket(t, root, "story", "Login", "--repo", "app", "--requirements", "R1")
	editTicket(t, root, "S-0001", "requirements: [R1]", "requirements: [R1, R99]")
	approveThrough(t, root, "architecture")
	code, stderr := gateStderr(t, root, "breakdown", "approved")
	want := "  specs/1-auth-flow/tickets/S-0001.md: S-0001: cites R99, which isn't in 1-auth-flow's spec\n"
	if code != 1 || !strings.Contains(stderr, want) {
		t.Fatalf("code = %d, want 1 naming R99:\n%s", code, stderr)
	}
	// Fixed: the uncovered R2 is only a warning.
	editTicket(t, root, "S-0001", "requirements: [R1, R99]", "requirements: [R1]")
	if code, stderr := gateStderr(t, root, "breakdown", "approved"); code != 0 {
		t.Fatalf("with only warnings = %d:\n%s", code, stderr)
	}
}
