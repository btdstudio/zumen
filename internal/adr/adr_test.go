package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateWritesNumberedADR(t *testing.T) {
	root := t.TempDir()
	rec, err := Create(root, "", Options{Title: "Use Postgres", Deciders: "architect", Context: "need a store", Decision: "Postgres", Consequences: "ops burden"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Number != 1 || rec.Slug != "use-postgres" || rec.Status != "proposed" {
		t.Fatalf("rec = %+v", rec)
	}
	if !strings.HasSuffix(rec.Path, filepath.Join("04_architecture", "adr", "0001-use-postgres.md")) {
		t.Fatalf("path = %q", rec.Path)
	}
	raw, _ := os.ReadFile(rec.Path)
	s := string(raw)
	for _, want := range []string{
		"# 0001 — Use Postgres",
		"- Status: proposed",
		"- Deciders: architect",
		"## Context", "need a store",
		"## Decision", "Postgres",
		"## Consequences", "ops burden",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
}

func TestNumbersAreSequential(t *testing.T) {
	root := t.TempDir()
	a, _ := Create(root, "", Options{Title: "a"})
	b, _ := Create(root, "", Options{Title: "b"})
	c, _ := Create(root, "", Options{Title: "c"})
	if a.Number != 1 || b.Number != 2 || c.Number != 3 {
		t.Fatalf("numbers = %d,%d,%d", a.Number, b.Number, c.Number)
	}
}

func TestListSortedSkippingNonNumeric(t *testing.T) {
	root := t.TempDir()
	Create(root, "", Options{Title: "first"})
	Create(root, "", Options{Title: "second"})
	os.WriteFile(filepath.Join(root, "04_architecture", "adr", "README.md"), []byte("x"), 0o644)
	list := List(root, "")
	if len(list) != 2 || list[0].Number != 1 || list[1].Number != 2 {
		t.Fatalf("list = %+v", list)
	}
}

func TestShowAndMissing(t *testing.T) {
	root := t.TempDir()
	Create(root, "", Options{Title: "T"})
	got, err := Show(root, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "T" {
		t.Fatalf("got = %+v", got)
	}
	if _, err := Show(root, "", 99); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestParseExtractsFields(t *testing.T) {
	raw := "# 0003 — Split the monolith\n\n- Status: accepted\n- Date: 2026-01-02\n- Deciders: architect\n\n## Context\n\n## Decision\n\n## Consequences\n"
	rec := parse(raw, 3, "split-the-monolith")
	if rec.Number != 3 || rec.Slug != "split-the-monolith" || rec.Title != "Split the monolith" ||
		rec.Status != "accepted" || rec.Date != "2026-01-02" || rec.Deciders != "architect" {
		t.Fatalf("rec = %+v", rec)
	}
}

func TestAdrDirDefault(t *testing.T) {
	root := t.TempDir()
	if AdrDir(root, "") != filepath.Join(root, "04_architecture", "adr") {
		t.Fatalf("dir = %q", AdrDir(root, ""))
	}
}

func TestNextNumberEmpty(t *testing.T) {
	root := t.TempDir()
	if NextNumber(root, "") != 1 {
		t.Fatalf("next = %d", NextNumber(root, ""))
	}
}

func TestCreateWithBody(t *testing.T) {
	root := t.TempDir()
	body := "## Context\n\nwhy\n\n## Decision\n\nwhat\n\n## Consequences\n\ntradeoff\n"
	rec, err := Create(root, "", Options{Title: "Big decision", Deciders: "architect", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(rec.Path)
	s := string(raw)
	if !strings.Contains(s, "# 0001 — Big decision") || !strings.Contains(s, "why") || !strings.Contains(s, "tradeoff") {
		t.Fatalf("body not written:\n%s", s)
	}
}

// Slug truncation (T-0031).

// longTitle's slug is 39 letters, a "-", then more, so the 40-rune cut lands
// just after the "-".
var longTitle = strings.Repeat("a", 39) + " then more words"

// AC1, AC4: the cut never leaves a trailing "-"; other slugs are unchanged.
func TestSlugifyTrimsTrailingDashAfterCut(t *testing.T) {
	for _, tc := range []struct{ title, want string }{
		{longTitle, strings.Repeat("a", 39)},
		{strings.Repeat("b", 45), strings.Repeat("b", 40)},
		{"Canonical ticket content hash for freeze!", "canonical-ticket-content-hash-for-freeze"},
		{"Use Postgres", "use-postgres"},
		{"!!!", ""},
	} {
		if got := slugify(tc.title); got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

// AC1, AC2: a cut slug's file matches, so the next ADR numbers on.
func TestCreateCutSlugMatchesAndNumbersOn(t *testing.T) {
	root := t.TempDir()
	a, err := Create(root, "", Options{Title: longTitle})
	if err != nil {
		t.Fatal(err)
	}
	if name := filepath.Base(a.Path); !adrFileRe.MatchString(name) || name != "0001-"+strings.Repeat("a", 39)+".md" {
		t.Fatalf("name = %q", name)
	}
	b, err := Create(root, "", Options{Title: longTitle + " again"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Number != 2 || len(List(root, "")) != 2 {
		t.Fatalf("second = %d, list = %d", b.Number, len(List(root, "")))
	}
}

// AC3: a legacy name ending in "-" still holds its number; List skips it.
// Only .md files count.
func TestNextNumberCountsMalformedNames(t *testing.T) {
	root := t.TempDir()
	dir := AdrDir(root, "")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0003-some-title-.md"), []byte("# 0003 — Some title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A numbered file that isn't .md doesn't count.
	if err := os.WriteFile(filepath.Join(dir, "0009-notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec, err := Create(root, "", Options{Title: "Next"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Number != 4 {
		t.Fatalf("number = %d, want 4", rec.Number)
	}
	if list := List(root, ""); len(list) != 1 || list[0].Number != 4 {
		t.Fatalf("list = %+v", list)
	}
}
