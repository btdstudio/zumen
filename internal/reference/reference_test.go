package reference

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseIndex(t *testing.T) {
	text := "# References\n\nSome prose.\n\n" +
		"- `design/a.md` — Design A\n" +
		"  - purpose: why it matters\n" +
		"  - from: Brian\n" +
		"  - added: 2026-11-12\n" +
		"  - summary: a short one\n" +
		"  - colour: ignored\n" +
		"\n## More\n\n" +
		"- `b.png`\n" +
		"  - purpose: a picture\n" +
		"- `./c//d.txt` - Plain title\n" +
		"  - from: AI chat, 2026-11-10\n" +
		"- not an entry\n" +
		"  - purpose: orphan field after a plain item\n"
	got := ParseIndex(text)
	if len(got) != 3 {
		t.Fatalf("entries = %+v, want 3", got)
	}
	a := got[0]
	if a.Path != "design/a.md" || a.Title != "Design A" || a.Line != 5 || a.Fields["purpose"] != "why it matters" ||
		a.Fields["from"] != "Brian" || a.Fields["added"] != "2026-11-12" || a.Fields["summary"] != "a short one" || len(a.Fields) != 4 {
		t.Errorf("entry a = %+v", a)
	}
	if b := got[1]; b.Path != "b.png" || b.Title != "" || b.Fields["purpose"] != "a picture" || len(b.Fields) != 1 {
		t.Errorf("entry b = %+v", b)
	}
	// The heading ended b's entry, so nothing after it attaches to a plain item,
	// and a path is cleaned.
	if c := got[2]; c.Path != "c/d.txt" || c.Title != "Plain title" || c.Fields["from"] != "AI chat, 2026-11-10" || len(c.Fields) != 1 {
		t.Errorf("entry c = %+v", c)
	}
	if len(ParseIndex("")) != 0 || len(ParseIndex("just words\n- plain\n")) != 0 {
		t.Error("text with no entries gave entries")
	}
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

func messages(res Result) []string {
	var out []string
	for _, f := range res.Findings {
		out = append(out, f.Message)
	}
	return out
}

const goodEntry = "  - purpose: p\n  - from: me\n  - added: 2026-11-12\n"

// AC2: an indexed markdown file, an indexed image with a summary, an unindexed
// file and an entry with no file; index.md, context.md and dotfiles are ignored.
func TestCheck(t *testing.T) {
	root := t.TempDir()
	write(t, root, "references/index.md", "# Index\n\n"+
		"- `notes.md` — Notes\n"+goodEntry+
		"- `design/diagram.png` — Diagram\n"+goodEntry+"  - summary: a flow with three boxes\n"+
		"- `gone.md` — Gone\n"+goodEntry)
	write(t, root, "references/context.md", "the guide")
	write(t, root, "references/notes.md", "notes")
	write(t, root, "references/design/diagram.png", "png")
	write(t, root, "references/stray.pdf", "pdf")
	write(t, root, "references/.DS_Store", "x")
	write(t, root, "references/.hidden/inside.md", "x")
	res, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"gone.md: listed in the index, but the file is missing",
		"stray.pdf: not in references/index.md",
	}
	if got := messages(res); !slices.Equal(got, want) {
		t.Fatalf("findings = %q, want %q", got, want)
	}
	if !res.Present || res.Files != 3 || res.Indexed != 2 {
		t.Errorf("result = %+v, want present, 3 files, 2 indexed", res)
	}
}

// AC3: a missing required field names the entry and the field; an empty value
// counts as missing; a missing index leaves every file unindexed.
func TestCheckFieldsAndMissingIndex(t *testing.T) {
	root := t.TempDir()
	write(t, root, "references/index.md", "- `a.md`\n  - purpose: p\n  - added: 2026-11-12\n- `b.md`\n  - purpose:\n  - from: me\n  - added: x\n")
	write(t, root, "references/a.md", "a")
	write(t, root, "references/b.md", "b")
	res, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := messages(res), []string{"a.md: missing from", "b.md: missing purpose"}; !slices.Equal(got, want) {
		t.Fatalf("findings = %q, want %q", got, want)
	}

	bare := t.TempDir()
	write(t, bare, "references/context.md", "guide")
	write(t, bare, "references/one.md", "1")
	write(t, bare, "references/sub/two.md", "2")
	write(t, bare, "references/sub/index.md", "a nested index.md is a user file")
	res, err = Check(bare)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"one.md: not in references/index.md",
		"sub/index.md: not in references/index.md",
		"sub/two.md: not in references/index.md",
	}
	if got := messages(res); !slices.Equal(got, want) || res.Indexed != 0 || res.Files != 3 {
		t.Fatalf("no index: findings = %q, result = %+v", got, res)
	}
}

func TestCheckOutsideAndAbsentFolder(t *testing.T) {
	root := t.TempDir()
	if res, err := Check(root); err != nil || res.Present || len(res.Findings) != 0 {
		t.Fatalf("no folder: %+v, %v", res, err)
	}
	write(t, root, "specs/secret.md", "not a reference")
	write(t, root, "references/index.md", "- `../specs/secret.md`\n"+goodEntry+"- `/etc/passwd`\n"+goodEntry+"- `..`\n"+goodEntry)
	res, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(messages(res), "|")
	for _, want := range []string{"../specs/secret.md: outside references/", "/etc/passwd: outside references/", "..: outside references/"} {
		if !strings.Contains(got, want) {
			t.Errorf("findings %q lack %q", got, want)
		}
	}
	if strings.Contains(got, "missing") {
		t.Errorf("an outside path was looked up on disk: %q", got)
	}
}
