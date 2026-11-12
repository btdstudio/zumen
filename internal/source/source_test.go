package source

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestWriteNewSourceCreatesIssueFrontmatter(t *testing.T) {
	root := t.TempDir()
	created, err := WriteNewSource(root, "", NewSourceOptions{Title: "Build the thing", Source: SOURCE_ISSUE, Body: "Wire up the store."})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 1 || created.Source != "issue" || created.Title != "Build the thing" || created.Body != "Wire up the store." {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasSuffix(created.Path, filepath.Join("00_intake", "1.md")) {
		t.Fatalf("path = %q", created.Path)
	}
	saved, _ := os.ReadFile(created.Path)
	want := "---\nid: 1\ntitle: Build the thing\nsource: issue\n---\n\nWire up the store.\n"
	if string(saved) != want {
		t.Fatalf("saved = %q, want %q", saved, want)
	}
}

func TestWriteNewSourceCreatesRepoFrontmatter(t *testing.T) {
	root := t.TempDir()
	created, err := WriteNewSource(root, "", NewSourceOptions{Title: "Refactor DB", Source: SOURCE_REPO, Repo: "~/code/app", Rev: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Source != "repo" || created.Repo != "~/code/app" || created.Rev != "main" {
		t.Fatalf("created = %+v", created)
	}
	saved, _ := os.ReadFile(created.Path)
	want := "---\nid: 1\ntitle: Refactor DB\nsource: repo\nrepo: ~/code/app\nrev: main\n---\n\n\n"
	if string(saved) != want {
		t.Fatalf("saved = %q, want %q", saved, want)
	}
}

func TestIDsAreSequential(t *testing.T) {
	root := t.TempDir()
	a, _ := WriteNewSource(root, "", NewSourceOptions{Title: "a"})
	b, _ := WriteNewSource(root, "", NewSourceOptions{Title: "b"})
	c, _ := WriteNewSource(root, "", NewSourceOptions{Title: "c"})
	if a.ID != 1 || b.ID != 2 || c.ID != 3 {
		t.Fatalf("ids = %d,%d,%d", a.ID, b.ID, c.ID)
	}
}

func TestListSourcesSortedSkippingNonNumeric(t *testing.T) {
	root := t.TempDir()
	WriteNewSource(root, "", NewSourceOptions{Title: "first"})
	WriteNewSource(root, "", NewSourceOptions{Title: "second"})
	os.WriteFile(filepath.Join(root, "00_intake", "README.md"), []byte("not a source\n"), 0o644)
	list := ListSources(root, "")
	if len(list) != 2 || list[0].ID != 1 || list[1].ID != 2 || list[0].Title != "first" || list[1].Title != "second" {
		t.Fatalf("list = %+v", list)
	}
}

func TestReadSourceAndMissing(t *testing.T) {
	root := t.TempDir()
	created, _ := WriteNewSource(root, "", NewSourceOptions{Title: "T", Source: SOURCE_REPO, Repo: "~/x", Body: "Body"})
	got, err := ReadSource(root, "", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "T" || got.Body != "Body" || got.Source != "repo" || got.Repo != "~/x" {
		t.Fatalf("got = %+v", got)
	}
	if _, err := ReadSource(root, "", 99); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestSourceDirRelocatesStore(t *testing.T) {
	root := t.TempDir()
	created, _ := WriteNewSource(root, "tickets", NewSourceOptions{Title: "x"})
	if !strings.HasSuffix(created.Path, filepath.Join("tickets", "1.md")) {
		t.Fatalf("path = %q", created.Path)
	}
	if SourceDir(root, "tickets") != filepath.Join(root, "tickets") {
		t.Fatalf("dir = %q", SourceDir(root, "tickets"))
	}
}

func TestEmptyStore(t *testing.T) {
	root := t.TempDir()
	if len(ListSources(root, "")) != 0 || NextId(root, "") != 1 {
		t.Fatalf("empty store list/next = %v / %d", ListSources(root, ""), NextId(root, ""))
	}
}

func TestWriteSourceRefusesToClobber(t *testing.T) {
	root := t.TempDir()
	dir := SourceDir(root, "")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "5.md"), []byte("existing content"), 0o644)
	_, err := WriteSource(root, "", Source{ID: 5, Source: SOURCE_ISSUE, Title: "x"})
	if err == nil || !strings.Contains(err.Error(), "not overwriting") {
		t.Fatalf("want not overwriting, got %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "5.md")); string(b) != "existing content" {
		t.Fatalf("5.md clobbered")
	}
}

func TestParseSourceRepoAndIssue(t *testing.T) {
	repo := ParseSource("---\nid: 2\ntitle: R\nsource: repo\nrepo: ~/code\nrev: dev\n---\n\nbody", 0)
	if repo.Source != "repo" || repo.Repo != "~/code" || repo.Rev != "dev" || repo.Body != "body" {
		t.Fatalf("repo parse = %+v", repo)
	}
	issue := ParseSource("---\nid: 1\ntitle: I\nsource: issue\n---\n", 0)
	if issue.Source != "issue" || issue.Repo != "" || issue.Rev != "" {
		t.Fatalf("issue parse = %+v", issue)
	}
}

func TestMaxRecordedId(t *testing.T) {
	recorded := []string{
		"00_intake/3.md",
		"./00_intake/5.md",
		"00_intake//4.md",
		"other/9.md",
		"issue:12",
		"https://example.com/11.md",
		"00_intake/7.txt",
		"00_intake/x.md",
		"00_intake/sub/8.md",
	}
	if got := MaxRecordedId("00_intake", recorded); got != 5 {
		t.Fatalf("MaxRecordedId = %d, want 5", got)
	}
	if got := MaxRecordedId("", []string{"00_intake/6.md"}); got != 6 {
		t.Fatalf("default sourceDir = %d, want 6", got)
	}
	if got := MaxRecordedId("tickets", recorded); got != 0 {
		t.Fatalf("no match = %d, want 0", got)
	}
	if got := MaxRecordedId("00_intake", nil); got != 0 {
		t.Fatalf("none = %d, want 0", got)
	}
}

func TestWriteNewSourceRespectsAfter(t *testing.T) {
	for _, tc := range []struct{ after, want int }{{5, 6}, {1, 3}, {2, 3}, {0, 3}} {
		root := t.TempDir()
		WriteNewSource(root, "", NewSourceOptions{Title: "a"})
		WriteNewSource(root, "", NewSourceOptions{Title: "b"})
		created, err := WriteNewSource(root, "", NewSourceOptions{Title: "c", After: tc.after})
		if err != nil {
			t.Fatal(err)
		}
		if created.ID != tc.want {
			t.Fatalf("After %d: id = %d, want %d", tc.after, created.ID, tc.want)
		}
	}
}

// AC1: the write is the existence check, so nothing already at the path is ever
// opened for writing. A stat-then-write would write through a dangling symlink.
func TestWriteSourceNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	dir := SourceDir(root, "")
	os.MkdirAll(dir, 0o755)
	existing := filepath.Join(dir, "1.md")
	os.WriteFile(existing, []byte("sentinel"), 0o644)
	_, err := WriteSource(root, "", Source{ID: 1, Source: SOURCE_ISSUE, Title: "x"})
	if err == nil || !strings.Contains(err.Error(), "source 1 already exists") || !strings.Contains(err.Error(), "not overwriting") {
		t.Fatalf("existing file: err = %v", err)
	}
	if b, _ := os.ReadFile(existing); string(b) != "sentinel" {
		t.Fatalf("1.md = %q, want it untouched", b)
	}

	target := filepath.Join(t.TempDir(), "elsewhere.md")
	if err := os.Symlink(target, filepath.Join(dir, "2.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err = WriteSource(root, "", Source{ID: 2, Source: SOURCE_ISSUE, Title: "x"})
	if err == nil || !strings.Contains(err.Error(), "not overwriting") {
		t.Fatalf("dangling symlink: err = %v, want not overwriting", err)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("wrote through the symlink and created its target")
	}
}

func TestWriteSourceRejectsNonPositiveIds(t *testing.T) {
	for _, id := range []int{0, -1, math.MinInt} {
		root := t.TempDir()
		_, err := WriteSource(root, "", Source{ID: id, Source: SOURCE_ISSUE, Title: "x"})
		if err == nil || !strings.Contains(err.Error(), "out of range") {
			t.Fatalf("id %d: err = %v, want out of range", id, err)
		}
		if _, err := os.Stat(SourceDir(root, "")); err == nil {
			entries, _ := os.ReadDir(SourceDir(root, ""))
			if len(entries) != 0 {
				t.Fatalf("id %d: wrote %v", id, entries)
			}
		}
	}
}

// AC2: an id that would overflow is refused before anything is written, whether
// it comes from an intake file or from After.
func TestWriteNewSourceRefusesOverflow(t *testing.T) {
	largest := strconv.Itoa(math.MaxInt)
	for name, tc := range map[string]struct {
		file  string
		after int
	}{
		"file":  {file: largest + ".md"},
		"after": {after: math.MaxInt},
	} {
		root := t.TempDir()
		dir := SourceDir(root, "")
		os.MkdirAll(dir, 0o755)
		if tc.file != "" {
			os.WriteFile(filepath.Join(dir, tc.file), []byte("x"), 0o644)
		}
		before, _ := os.ReadDir(dir)
		_, err := WriteNewSource(root, "", NewSourceOptions{Title: "t", After: tc.after})
		if err == nil || !strings.Contains(err.Error(), "intake id out of range") {
			t.Fatalf("%s: err = %v, want intake id out of range", name, err)
		}
		if after, _ := os.ReadDir(dir); len(after) != len(before) {
			t.Fatalf("%s: wrote a file: %v", name, after)
		}
	}
	// One below the largest still works.
	root := t.TempDir()
	created, err := WriteNewSource(root, "", NewSourceOptions{Title: "t", After: math.MaxInt - 2})
	if err != nil || created.ID != math.MaxInt-1 {
		t.Fatalf("After MaxInt-2: id = %d, err = %v; want MaxInt-1", created.ID, err)
	}
	// The largest id itself is still allowed; only the one after it is refused.
	root = t.TempDir()
	created, err = WriteNewSource(root, "", NewSourceOptions{Title: "t", After: math.MaxInt - 1})
	if err != nil || created.ID != math.MaxInt {
		t.Fatalf("After MaxInt-1: id = %d, err = %v; want MaxInt", created.ID, err)
	}
}

// An open that fails for a reason other than "exists" is reported as is, and
// leaves nothing behind; so does a store folder that can't be created.
func TestWriteSourceReportsOtherErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := t.TempDir()
	dir := SourceDir(root, "")
	os.MkdirAll(dir, 0o755)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	_, err := WriteSource(root, "", Source{ID: 1, Source: SOURCE_ISSUE, Title: "x"})
	if err == nil || strings.Contains(err.Error(), "not overwriting") {
		t.Fatalf("read-only store: err = %v, want a permission error, not 'not overwriting'", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("wrote %v", entries)
	}

	blocked := t.TempDir()
	os.WriteFile(filepath.Join(blocked, DEFAULT_SOURCE_DIR), []byte("x"), 0o644) // a file where the store folder goes
	if _, err := WriteSource(blocked, "", Source{ID: 1, Source: SOURCE_ISSUE, Title: "x"}); err == nil {
		t.Fatal("WriteSource succeeded with a file in place of the store folder")
	}
}
