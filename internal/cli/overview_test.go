package cli

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/zumen/internal/state"
)

var updateOverview = flag.Bool("update-overview", false, "update the overview golden")

const overviewReadme = "specs/1-auth-flow/README.md"

// overviewFixture copies testdata/overview/workspace into a temp dir.
func overviewFixture(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src := filepath.Join("..", "..", "testdata", "overview", "workspace")
	root := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		dst := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func overviewRun(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var out string
	code, stderr := captureStderr(t, func() int {
		c, o := captureStdout(t, func() int { return runOverview(root, args) })
		out = o
		return c
	})
	return code, out, stderr
}

func readme(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(overviewReadme)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// AC1: two runs give byte-identical files that match the golden.
func TestOverviewGolden(t *testing.T) {
	root := overviewFixture(t)
	code, out, stderr := overviewRun(t, root, "1-auth-flow")
	if code != 0 || out != "wrote "+overviewReadme+"\n" {
		t.Fatalf("first run = %d, %q, %q", code, out, stderr)
	}
	first := readme(t, root)
	code, out, _ = overviewRun(t, root, "1-auth-flow")
	if code != 0 || out != overviewReadme+" is up to date\n" || readme(t, root) != first {
		t.Fatalf("second run = %d, %q, or the file changed", code, out)
	}
	golden := filepath.Join("..", "..", "testdata", "golden", "overview", "README.md")
	if *updateOverview {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(first), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v (run `go test ./internal/cli -run TestOverviewGolden -update-overview`)", err)
	}
	if first != string(want) {
		t.Fatalf("overview differs from the golden; run `go test ./internal/cli -run TestOverviewGolden -update-overview` and review the diff")
	}
	// Another copy gives the same bytes: nothing depends on the temp dir.
	if other := overviewFixture(t); func() string { overviewRun(t, other, "1-auth-flow"); return readme(t, other) }() != first {
		t.Fatal("a second workspace copy rendered differently")
	}
}

// AC2: hand edits outside the generated region survive.
func TestOverviewKeepsHandEdits(t *testing.T) {
	root := overviewFixture(t)
	overviewRun(t, root, "1-auth-flow")
	edited := strings.Replace(readme(t, root), "# Auth flow\n", "# Auth flow\r\n\r\nA note above, by hand.\r\n", 1)
	edited = strings.Replace(edited, "<!-- About 250 words", "Sign-in, written by a person.\n<!-- About 250 words", 1)
	edited += "\nNotes below, by hand.\n"
	gen := strings.Index(edited, "<!-- zumen:generated")
	stale := edited[:gen] + strings.Replace(edited[gen:], "## Status", "## Status\n\nstale text", 1)
	path := filepath.Join(root, filepath.FromSlash(overviewReadme))
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := overviewRun(t, root, "1-auth-flow"); code != 0 || !strings.HasPrefix(out, "wrote ") {
		t.Fatalf("run = %d, %q", code, out)
	}
	got := readme(t, root)
	// Everything outside the generated region is the edited file's, byte for byte.
	outside := func(s string) (string, string) {
		start := strings.Index(s, "<!-- zumen:generated")
		end := strings.Index(s, "<!-- /zumen:generated")
		return s[:start], s[end:]
	}
	gotBefore, gotAfter := outside(got)
	wantBefore, wantAfter := outside(edited)
	if gotBefore != wantBefore || gotAfter != wantAfter {
		t.Fatalf("outside the region:\ngot  %q … %q\nwant %q … %q", gotBefore, gotAfter, wantBefore, wantAfter)
	}
	// The region was refreshed: the stale text is gone, and the narrative now
	// has words, so it isn't reported empty.
	if strings.Contains(got, "stale text") || strings.Contains(got, "narrative is empty") || !strings.Contains(got, "## Problems") {
		t.Fatalf("region not refreshed:\n%s", got)
	}
}

// AC3: malformed markers refuse, write nothing, and --check exits 2.
func TestOverviewMalformedMarkers(t *testing.T) {
	root := overviewFixture(t)
	overviewRun(t, root, "1-auth-flow")
	path := filepath.Join(root, filepath.FromSlash(overviewReadme))
	dup := strings.Replace(readme(t, root), "<!-- /zumen:generated -->", "<!-- zumen:generated again -->\n<!-- /zumen:generated -->", 1)
	if err := os.WriteFile(path, []byte(dup), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := overviewRun(t, root, "1-auth-flow")
	if code != 1 || out != "" || !strings.Contains(stderr, overviewReadme+": duplicated marker <!-- zumen:generated -->") {
		t.Fatalf("write = %d, %q, %q", code, out, stderr)
	}
	if readme(t, root) != dup {
		t.Fatal("a refused run changed the file")
	}
	if code, _, stderr := overviewRun(t, root, "1-auth-flow", "--check"); code != 2 || !strings.Contains(stderr, "duplicated marker") {
		t.Fatalf("--check = %d, %q", code, stderr)
	}
	if code, out, _ := overviewRun(t, root, "1-auth-flow", "--stdout"); code != 1 || out != "" {
		t.Fatalf("--stdout = %d, %q", code, out)
	}
}

// AC4: --check is 0 when current, 1 after a ticket title changes, and writes
// nothing.
func TestOverviewCheck(t *testing.T) {
	root := overviewFixture(t)
	if code, out, _ := overviewRun(t, root, "1-auth-flow", "--check"); code != 1 || !strings.Contains(out, "is out of date") {
		t.Fatalf("missing file: --check = %d, %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(overviewReadme))); err == nil {
		t.Fatal("--check created the file")
	}
	overviewRun(t, root, "1-auth-flow")
	if code, out, _ := overviewRun(t, root, "1-auth-flow", "--check"); code != 0 || out != overviewReadme+" is up to date\n" {
		t.Fatalf("current: --check = %d, %q", code, out)
	}
	before := readme(t, root)
	ticketFile := filepath.Join(root, "specs", "1-auth-flow", "tickets", "T-0007.md")
	data, err := os.ReadFile(ticketFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ticketFile, []byte(strings.Replace(string(data), "title: Rotate signing keys", "title: Rotate all keys", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := overviewRun(t, root, "1-auth-flow", "--check"); code != 1 || !strings.Contains(out, "is out of date — run zumen overview") {
		t.Fatalf("stale: --check = %d, %q", code, out)
	}
	if readme(t, root) != before {
		t.Fatal("--check changed the file")
	}
}

// AC8: --stdout prints the file and writes nothing.
func TestOverviewStdout(t *testing.T) {
	root := overviewFixture(t)
	code, out, _ := overviewRun(t, root, "1-auth-flow", "--stdout")
	if code != 0 || !strings.HasPrefix(out, "# Auth flow\n") || !strings.Contains(out, "## Problems") {
		t.Fatalf("--stdout = %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(overviewReadme))); err == nil {
		t.Fatal("--stdout wrote the file")
	}
	overviewRun(t, root, "1-auth-flow")
	if out != readme(t, root) {
		t.Fatal("--stdout differs from the written file")
	}
}

// AC9: the active change by default; unknown names and bad flags are refused.
func TestOverviewArguments(t *testing.T) {
	root := overviewFixture(t)
	if code, _, stderr := overviewRun(t, root); code != 1 || !strings.Contains(stderr, "no active change") {
		t.Fatalf("no active change = %d, %q", code, stderr)
	}
	if _, err := state.SwitchTo(root, "1-auth-flow"); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := overviewRun(t, root); code != 0 || out != "wrote "+overviewReadme+"\n" {
		t.Fatalf("active change = %d, %q", code, out)
	}
	if !strings.HasPrefix(readme(t, root), "# Auth flow\n\n<!-- zumen:narrative -->\n") {
		t.Fatalf("stub title:\n%s", readme(t, root))
	}
	for _, args := range [][]string{{"nope"}, {"../x"}, {"specs/1-auth-flow"}} {
		if code, _, stderr := overviewRun(t, root, args...); code != 1 || !strings.Contains(stderr, "no change named") {
			t.Fatalf("%v = %d, %q", args, code, stderr)
		}
	}
	if code, _, stderr := overviewRun(t, root, "nope", "--check"); code != 2 || !strings.Contains(stderr, "no change named") {
		t.Fatalf("--check unknown = %d, %q", code, stderr)
	}
	for _, args := range [][]string{{"--check", "--stdout"}, {"--force"}, {"1-auth-flow", "2-other-work"}} {
		if code, _, stderr := overviewRun(t, root, args...); code != 1 || !strings.Contains(stderr, usageLine("overview")) {
			t.Fatalf("%v = %d, %q", args, code, stderr)
		}
	}
	if code, _, _ := overviewRun(t, root, "2-other-work"); code != 0 {
		t.Fatal("another named change")
	}
}
