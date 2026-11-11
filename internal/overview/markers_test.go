package overview

import (
	"strings"
	"testing"
)

func TestStubMerges(t *testing.T) {
	got, err := Merge(Stub("Auth flow"), "\nREGION\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "# Auth flow\n\n" +
		"<!-- zumen:narrative -->\n" +
		"<!-- About 250 words: the problem, the shape of the solution, who's affected, and what's out of scope. -->\n" +
		"<!-- /zumen:narrative -->\n\n" +
		"<!-- zumen:generated — do not edit; run `zumen overview` -->\n" +
		"\nREGION\n" +
		"<!-- /zumen:generated -->\n"
	if string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// AC2: everything outside the generated region is kept byte for byte.
func TestMergeKeepsHandEdits(t *testing.T) {
	before := "Intro written by hand.\r\n\r\n" +
		"<!-- zumen:narrative -->\r\nOur story, edited.\r\n<!-- /zumen:narrative -->\r\n" +
		"Between the regions.\n" +
		"<!-- zumen:generated (older wording) -->\nold generated\nlines\n<!-- /zumen:generated -->\n" +
		"After, with no trailing newline"
	got, err := Merge([]byte(before), "\nnew\n")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(before, "\nold generated\nlines\n", "\n\nnew\n", 1)
	if string(got) != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
	n, err := Narrative([]byte(before))
	if err != nil || n != "Our story, edited.\r\n" {
		t.Fatalf("narrative = %q, %v", n, err)
	}
}

// AC3: a missing, duplicated or out-of-order marker is refused, naming it.
func TestMergeMarkerErrors(t *testing.T) {
	const (
		ns = "<!-- zumen:narrative -->\n"
		ne = "<!-- /zumen:narrative -->\n"
		gs = "<!-- zumen:generated -->\n"
		ge = "<!-- /zumen:generated -->\n"
	)
	cases := []struct{ name, doc, want string }{
		{"missing narrative start", ne + gs + ge, "missing marker <!-- zumen:narrative -->"},
		{"missing narrative end", ns + gs + ge, "missing marker <!-- /zumen:narrative -->"},
		{"missing generated start", ns + ne + ge, "missing marker <!-- zumen:generated -->"},
		{"missing generated end", ns + ne + gs, "missing marker <!-- /zumen:generated -->"},
		{"duplicated generated start", ns + ne + gs + "x\n" + gs + ge, "duplicated marker <!-- zumen:generated --> (lines 3 and 5)"},
		{"duplicated narrative end", ns + ne + ne + gs + ge, "duplicated marker <!-- /zumen:narrative --> (lines 2 and 3)"},
		{"narrative end first", ne + ns + gs + ge, "markers out of order: <!-- /zumen:narrative --> (line 1) comes before <!-- zumen:narrative --> (line 2)"},
		{"generated before narrative", gs + ge + ns + ne, "markers out of order: <!-- zumen:generated --> (line 1) comes before <!-- /zumen:narrative --> (line 4)"},
		{"generated end first", ns + ne + ge + gs, "markers out of order: <!-- /zumen:generated --> (line 3) comes before <!-- zumen:generated --> (line 4)"},
		{"empty file", "", "missing marker <!-- zumen:narrative -->"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if out, err := Merge([]byte(c.doc), "\nx\n"); err == nil || err.Error() != c.want || out != nil {
				t.Fatalf("Merge = %q, %v; want %q", out, err, c.want)
			}
			if _, err := Narrative([]byte(c.doc)); err == nil || err.Error() != c.want {
				t.Fatalf("Narrative err = %v; want %q", err, c.want)
			}
		})
	}
}

func TestMarkersNeedTheirOwnLine(t *testing.T) {
	doc := "text <!-- zumen:narrative -->\n<!-- zumen:narrative -->\n<!-- /zumen:narrative -->\n<!-- zumen:generated -->\n<!-- /zumen:generated -->\n"
	if _, err := Merge([]byte(doc), "\n"); err != nil {
		t.Fatalf("a marker mid-line counted: %v", err)
	}
}

func TestNarrativeEmpty(t *testing.T) {
	for text, want := range map[string]bool{
		"":                                  true,
		"  \n\t\n":                          true,
		"<!-- About 250 words -->\n":        true,
		"<!-- a\nmulti-line prompt -->\n\n": true,
		"Real words.\n":                     false,
		"<!-- prompt -->\nReal words.\n":    false,
	} {
		if got := NarrativeEmpty(text); got != want {
			t.Errorf("NarrativeEmpty(%q) = %v", text, got)
		}
	}
	n, err := Narrative(Stub("X"))
	if err != nil || !NarrativeEmpty(n) {
		t.Fatalf("the stub's narrative = %q, %v; want empty", n, err)
	}
}
