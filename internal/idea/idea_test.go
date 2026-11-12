package idea

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const day1, day2, day3 = "2026-11-12", "2026-11-13", "2026-11-14"

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// bodyOf is everything after the frontmatter block.
func bodyOf(t *testing.T, raw string) string {
	t.Helper()
	parts := strings.SplitN(raw, "\n---\n", 2)
	if len(parts) != 2 {
		t.Fatalf("no frontmatter in %q", raw)
	}
	return parts[1]
}

func newIdea(t *testing.T, root, title string) Idea {
	t.Helper()
	i, err := New(root, title, day1)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

// AC2: a folder with the summary from the template and a log.
func TestNewCreatesTheFolderSummaryAndLog(t *testing.T) {
	root := t.TempDir()
	i := newIdea(t, root, "Faster imports")
	if i.ID != 1 || i.Slug != "faster-imports" || i.Dir != "ideas/1-faster-imports" || i.Status != Open || i.Title != "Faster imports" || i.Updated != day1 {
		t.Fatalf("idea = %+v", i)
	}
	summary := read(t, root, "ideas/1-faster-imports/summary.md")
	wantFront := "---\nid: 1\ntitle: Faster imports\nstatus: open\nreason:\nupdated: 2026-11-12\nchanges:\n---\n"
	if !strings.HasPrefix(summary, wantFront) {
		t.Fatalf("summary starts %q, want %q", summary[:min(len(summary), 120)], wantFront)
	}
	for _, section := range []string{"## Problem", "## Options considered", "## Cuts and no-gos", "## Open questions", "## References cited", "## Resulting changes"} {
		if !strings.Contains(summary, "\n"+section+"\n") {
			t.Errorf("summary lacks %q", section)
		}
	}
	if lower := strings.ToLower(summary); strings.Contains(lower, "appetite") || strings.Contains(lower, "budget") {
		t.Error("the summary has an appetite or time-budget field")
	}
	if log := read(t, root, "ideas/1-faster-imports/log.md"); log != "# Log\n\n- 2026-11-12 created\n" {
		t.Errorf("log = %q", log)
	}
}

// AC3: ids are sequential, and a deleted folder doesn't free its id.
func TestNewIdsAreSequentialAndNeverReused(t *testing.T) {
	root := t.TempDir()
	a, b, c := newIdea(t, root, "Same title"), newIdea(t, root, "Same title"), newIdea(t, root, "Third")
	if a.ID != 1 || b.ID != 2 || c.ID != 3 || b.Dir != "ideas/2-same-title" {
		t.Fatalf("ids = %d %d %d, dir %q", a.ID, b.ID, c.ID, b.Dir)
	}
	if err := os.RemoveAll(filepath.Join(root, "ideas", "2-same-title")); err != nil {
		t.Fatal(err)
	}
	if d := newIdea(t, root, "Fourth"); d.ID != 4 {
		t.Fatalf("with 1 and 3 left, the next id = %d, want 4", d.ID)
	}
}

// A3: only <n>-<slug> folders are ideas; the room's own files aren't.
func TestTheRoomsOwnFilesAreNotIdeas(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ideas/context.md", "the guide")
	write(t, root, "ideas/personas/facilitator.md", "persona")
	write(t, root, "ideas/templates/summary.md", "template")
	write(t, root, "ideas/9-notes.txt", "a file, not a folder")
	write(t, root, "ideas/drafts/x.md", "a folder with no number")
	if got, err := List(root); err != nil || len(got) != 0 {
		t.Fatalf("List = %v, %v; want none", got, err)
	}
	if i := newIdea(t, root, "First"); i.ID != 1 {
		t.Fatalf("id = %d, want 1 despite the stray names", i.ID)
	}
}

func TestNewEdgeCases(t *testing.T) {
	root := t.TempDir()
	if i := newIdea(t, root, "!!!"); i.Dir != "ideas/1-idea" || i.Title != "!!!" {
		t.Fatalf("an empty slug: %+v, want the idea fallback", i)
	}
	for _, title := range []string{"", "  \n\t "} {
		if _, err := New(root, title, day1); err == nil || !strings.Contains(err.Error(), "needs a title") {
			t.Errorf("title %q: err = %v, want a title error", title, err)
		}
	}
	// The largest id leaves no next id.
	big := t.TempDir()
	write(t, big, "ideas/9223372036854775807-x/summary.md", "---\nid: 1\n---\n")
	if _, err := New(big, "Next", day1); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Errorf("overflow: err = %v, want out of range", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(big, "ideas")); len(entries) != 1 {
		t.Errorf("overflow wrote %v", entries)
	}
	// A title can't add a frontmatter line.
	forged := newIdea(t, root, "Real\nstatus: dropped")
	raw := read(t, root, forged.Dir+"/summary.md")
	if forged.Title != "Real status: dropped" || forged.Status != Open || strings.Count(raw, "\nstatus:") != 1 {
		t.Errorf("title injection: %+v\n%s", forged, raw)
	}
	// An existing file is never overwritten.
	existing := filepath.Join(root, forged.Dir, "log.md")
	if err := createFile(existing, "x"); err == nil {
		t.Error("createFile overwrote an existing file")
	}
}

// AC4: park and drop set the status, reason and date, log a line, and leave the
// body byte for byte; a missing reason writes nothing.
func TestParkAndDrop(t *testing.T) {
	for _, tc := range []struct {
		name   string
		do     func(root string, id int, reason, today string) (Idea, error)
		status string
	}{{"park", Park, Parked}, {"drop", Drop, Dropped}} {
		root := t.TempDir()
		i := newIdea(t, root, "An idea")
		body := bodyOf(t, read(t, root, i.Dir+"/summary.md"))
		beforeSummary, beforeLog := read(t, root, i.Dir+"/summary.md"), read(t, root, i.Dir+"/log.md")

		for _, reason := range []string{"", "  \n "} {
			if _, err := tc.do(root, 1, reason, day2); err == nil || !strings.Contains(err.Error(), "reason is required") {
				t.Errorf("%s with reason %q: err = %v", tc.name, reason, err)
			}
		}
		if read(t, root, i.Dir+"/summary.md") != beforeSummary || read(t, root, i.Dir+"/log.md") != beforeLog {
			t.Fatalf("%s without a reason wrote something", tc.name)
		}

		got, err := tc.do(root, 1, "later", day2)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != tc.status || got.Reason != "later" || got.Updated != day2 {
			t.Fatalf("%s: %+v", tc.name, got)
		}
		raw := read(t, root, i.Dir+"/summary.md")
		for _, want := range []string{"status: " + tc.status, "reason: later", "updated: " + day2} {
			if !strings.Contains(raw, want) {
				t.Errorf("%s: summary lacks %q", tc.name, want)
			}
		}
		if bodyOf(t, raw) != body {
			t.Errorf("%s changed the body", tc.name)
		}
		if log := read(t, root, i.Dir+"/log.md"); log != beforeLog+"- "+day2+" "+tc.status+": later\n" {
			t.Errorf("%s: log = %q", tc.name, log)
		}

		// Only an open idea can be parked or dropped, and a refusal writes nothing.
		beforeSummary, beforeLog = raw, read(t, root, i.Dir+"/log.md")
		if _, err := tc.do(root, 1, "again", day3); err == nil || !strings.Contains(err.Error(), "idea 1 is "+tc.status+": only an open idea can be") {
			t.Errorf("%s twice: err = %v", tc.name, err)
		}
		if read(t, root, i.Dir+"/summary.md") != beforeSummary || read(t, root, i.Dir+"/log.md") != beforeLog {
			t.Errorf("%s: a refusal wrote something", tc.name)
		}
	}
}

// A2: a reason can't add a frontmatter line.
func TestAReasonCantInjectAFrontmatterLine(t *testing.T) {
	root := t.TempDir()
	i := newIdea(t, root, "An idea")
	got, err := Park(root, 1, "x\nstatus: open\nchanges: 9-forged", day2)
	if err != nil {
		t.Fatal(err)
	}
	raw := read(t, root, i.Dir+"/summary.md")
	front := strings.SplitN(raw, "\n---\n", 2)[0]
	if got.Status != Parked || got.Reason != "x status: open changes: 9-forged" || len(got.Changes) != 0 || strings.Count(front, "\nstatus:") != 1 || strings.Count(front, "\nchanges:") != 1 {
		t.Fatalf("reason injection: %+v\n%s", got, front)
	}
}

// AC5: reopen returns a parked or dropped idea to open and logs the reason.
func TestReopen(t *testing.T) {
	root := t.TempDir()
	i := newIdea(t, root, "An idea")
	if _, err := Reopen(root, 1, "now", day2); err == nil || !strings.Contains(err.Error(), "idea 1 is open: only a parked or dropped idea can be reopened") {
		t.Fatalf("reopen of an open idea: err = %v", err)
	}
	for _, park := range []func(string, int, string, string) (Idea, error){Park, Drop} {
		if _, err := park(root, 1, "not yet", day2); err != nil {
			t.Fatal(err)
		}
		body := bodyOf(t, read(t, root, i.Dir+"/summary.md"))
		if _, err := Reopen(root, 1, "", day3); err == nil || !strings.Contains(err.Error(), "reason is required") {
			t.Fatalf("reopen without a reason: err = %v", err)
		}
		got, err := Reopen(root, 1, "it matters again", day3)
		if err != nil {
			t.Fatal(err)
		}
		raw := read(t, root, i.Dir+"/summary.md")
		if got.Status != Open || got.Reason != "" || got.Updated != day3 || !strings.Contains(raw, "\nreason:\n") || bodyOf(t, raw) != body {
			t.Fatalf("reopened: %+v\n%s", got, raw)
		}
		if log := read(t, root, i.Dir+"/log.md"); !strings.HasSuffix(log, "- "+day3+" reopened: it matters again\n") {
			t.Errorf("log = %q", log)
		}
	}
}

// AC8: promotion records each change, in order, and leaves the body.
func TestMarkPromoted(t *testing.T) {
	root := t.TempDir()
	i := newIdea(t, root, "An idea")
	body := bodyOf(t, read(t, root, i.Dir+"/summary.md"))
	if _, err := MarkPromoted(root, 1, "", day2); err == nil {
		t.Error("an empty change name was accepted")
	}
	got, err := MarkPromoted(root, 1, "3-part-one", day2)
	if err != nil || got.Status != Promoted || !slices.Equal(got.Changes, []string{"3-part-one"}) {
		t.Fatalf("first promotion: %+v, %v", got, err)
	}
	got, err = MarkPromoted(root, 1, "4-part-two", day3)
	if err != nil || !slices.Equal(got.Changes, []string{"3-part-one", "4-part-two"}) || got.Updated != day3 {
		t.Fatalf("second promotion: %+v, %v", got, err)
	}
	raw := read(t, root, i.Dir+"/summary.md")
	if !strings.Contains(raw, "\nchanges: 3-part-one, 4-part-two\n") || bodyOf(t, raw) != body {
		t.Errorf("summary after promotion:\n%s", raw)
	}
	log := read(t, root, i.Dir+"/log.md")
	if !strings.Contains(log, "- "+day2+" promoted to 3-part-one\n") || !strings.HasSuffix(log, "- "+day3+" promoted to 4-part-two\n") {
		t.Errorf("log = %q", log)
	}
	// A promoted idea can't be parked or dropped, only promoted again.
	if _, err := Park(root, 1, "x", day3); err == nil || !strings.Contains(err.Error(), "idea 1 is promoted: only an open idea can be parked") {
		t.Errorf("park of a promoted idea: err = %v", err)
	}
	// A parked or dropped idea has to be reopened first, and nothing is written.
	for _, other := range []func(string, int, string, string) (Idea, error){Park, Drop} {
		j := newIdea(t, root, "Another")
		if _, err := other(root, j.ID, "not now", day2); err != nil {
			t.Fatal(err)
		}
		before := read(t, root, j.Dir+"/summary.md")
		if _, err := MarkPromoted(root, j.ID, "9-x", day3); err == nil || !strings.Contains(err.Error(), "reopen it first") {
			t.Errorf("promote of a parked or dropped idea: err = %v", err)
		}
		if read(t, root, j.Dir+"/summary.md") != before {
			t.Error("a refused promotion wrote the summary")
		}
	}
}

// N2: a hand-edited summary keeps its extra keys and body through a status change.
func TestHandEditsSurviveAStatusChange(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ideas/1-hand-made/summary.md", "---\nowner: brian\nid: 1\nTitle: Hand made\nstatus: open\nlabels: a, b\n---\n\n## Problem\n\nMine, with *markdown*.\n")
	write(t, root, "ideas/1-hand-made/log.md", "# Log\n\n- no final newline")
	got, err := Park(root, 1, "waiting", day2)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Hand made" || got.Status != Parked || got.Slug != "hand-made" {
		t.Fatalf("idea = %+v", got)
	}
	want := "---\nowner: brian\nid: 1\nTitle: Hand made\nstatus: parked\nlabels: a, b\nreason: waiting\nupdated: 2026-11-13\n---\n\n## Problem\n\nMine, with *markdown*.\n"
	if raw := read(t, root, "ideas/1-hand-made/summary.md"); raw != want {
		t.Errorf("summary =\n%q\nwant\n%q", raw, want)
	}
	if log := read(t, root, "ideas/1-hand-made/log.md"); log != "# Log\n\n- no final newline\n- 2026-11-13 parked: waiting\n" {
		t.Errorf("log = %q", log)
	}
}

func TestSetFrontmatter(t *testing.T) {
	for _, tc := range []struct{ name, raw, want string }{
		{"replaces in place", "---\na: 1\nb: 2\n---\nbody\n", "---\na: 1\nb: 9\n---\nbody\n"},
		{"adds a missing key before the closing line", "---\na: 1\n---\nbody", "---\na: 1\nb: 9\n---\nbody"},
		{"clears to a bare key", "---\nb: 2\n---\n", "---\nb:\n---\n"},
		{"keeps CRLF endings", "---\r\nb: 2\r\n---\r\nx\r\n", "---\r\nb: 9\r\n---\r\nx\r\n"},
		{"matches a key case-insensitively and keeps its spelling", "---\nB: 2\n---\n", "---\nB: 9\n---\n"},
		{"only the first matching line", "---\nb: 1\nb: 2\n---\n", "---\nb: 9\nb: 2\n---\n"},
		{"a --- later in the body isn't the block", "---\na: 1\n---\ntext\n---\nb: 5\n---\n", "---\na: 1\nb: 9\n---\ntext\n---\nb: 5\n---\n"},
	} {
		value := "9"
		if strings.Contains(tc.name, "clears") {
			value = ""
		}
		got, err := setFrontmatter(tc.raw, []kv{{"b", value}})
		if err != nil || got != tc.want {
			t.Errorf("%s: got %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
	for _, raw := range []string{"no frontmatter\n", "", "---\nunclosed: yes\n"} {
		if _, err := setFrontmatter(raw, []kv{{"b", "1"}}); err == nil {
			t.Errorf("setFrontmatter(%q) succeeded, want an error", raw)
		}
	}
}

func TestLoadAndListErrors(t *testing.T) {
	root := t.TempDir()
	if _, err := Load(root, 7); err == nil || !strings.Contains(err.Error(), "idea 7 not found") {
		t.Errorf("a missing idea: err = %v", err)
	}
	write(t, root, "ideas/1-fine/summary.md", "---\nid: 1\ntitle: Fine\nstatus: open\n---\n")
	write(t, root, "ideas/2-broken/summary.md", "no frontmatter at all\n")
	write(t, root, "ideas/3-a/summary.md", "---\ntitle: A\nstatus: open\n---\n")
	write(t, root, "ideas/3-b/summary.md", "---\ntitle: B\nstatus: open\n---\n")
	write(t, root, "ideas/4-odd/summary.md", "---\ntitle: Odd\nstatus: archived\n---\n")
	if _, err := Load(root, 2); err == nil || !strings.Contains(err.Error(), "ideas/2-broken/summary.md has no frontmatter") {
		t.Errorf("a broken summary: err = %v", err)
	}
	if _, err := Load(root, 3); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("two folders for one id: err = %v", err)
	}
	if _, err := Park(root, 4, "x", day2); err == nil || !strings.Contains(err.Error(), `unknown status "archived"`) {
		t.Errorf("an unknown status: err = %v", err)
	}
	if _, err := List(root); err == nil || !strings.Contains(err.Error(), "ideas/2-broken") {
		t.Errorf("List with a broken idea: err = %v, want it named", err)
	}
	if _, err := Load(root, 1); err != nil {
		t.Errorf("a fine idea: %v", err)
	}
}

func TestListIsOrderedAndComplete(t *testing.T) {
	root := t.TempDir()
	for _, title := range []string{"One", "Two", "Three", "Four"} {
		newIdea(t, root, title)
	}
	Park(root, 2, "later", day2)
	Drop(root, 3, "no", day2)
	MarkPromoted(root, 4, "5-four", day2)
	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	var summary []string
	for _, i := range got {
		summary = append(summary, strings.Join([]string{i.Title, i.Status, i.Reason}, "|"))
	}
	want := []string{"One|open|", "Two|parked|later", "Three|dropped|no", "Four|promoted|"}
	if !slices.Equal(summary, want) {
		t.Fatalf("List = %q, want %q", summary, want)
	}
}
