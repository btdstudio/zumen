package plan

import (
	"reflect"
	"strings"
	"testing"
)

// fenced returns the line numbers scan marks as fenced.
func fenced(markdown string) []int {
	var out []int
	for _, l := range scan(markdown) {
		if l.inFence {
			out = append(out, l.n)
		}
	}
	return out
}

func TestScanFences(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want []int
	}{
		{"backticks", "a\n```\nx\n```\nb", []int{2, 3, 4}},
		{"tildes with info", "~~~text\nx\n~~~\nb", []int{1, 2, 3}},
		{"longer close", "```\nx\n`````\nb", []int{1, 2, 3}},
		{"shorter run doesn't close", "````\n```\nx\n````\nb", []int{1, 2, 3, 4}},
		{"other char doesn't close", "```\n~~~\nx\n```\nb", []int{1, 2, 3, 4}},
		{"close with text doesn't close", "```\n``` go\nx\n```\nb", []int{1, 2, 3, 4}},
		{"indented, in a list item", "- a\n    ```\n    x\n    ```\nb", []int{2, 3, 4}},
		{"two ticks aren't a fence", "``\nx\n``", nil},
		{"backtick in info isn't a fence", "``` a`b\nx", nil},
		{"unclosed runs to the end", "a\n```\nx\n## y", []int{2, 3, 4}},
		{"crlf", "```\r\nx\r\n```\r\nb", []int{1, 2, 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := fenced(c.doc); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("fenced = %v, want %v", got, c.want)
			}
		})
	}
}

func TestScanTrimsCR(t *testing.T) {
	if got := scan("a\r\nb")[0].text; got != "a" {
		t.Fatalf("text = %q", got)
	}
}

func TestSectionLines(t *testing.T) {
	doc := strings.Join([]string{
		"# Title",          // 1
		"## Wanted  ",      // 2
		"a",                // 3
		"### Sub",          // 4
		"b",                // 5
		"```",              // 6
		"## not a heading", // 7
		"```",              // 8
		"## Other",         // 9
		"c",                // 10
		"## Wanted",        // 11
		"d",                // 12
		"# Ends too",       // 13
		"e",                // 14
	}, "\n")
	var got []int
	for _, l := range sectionLines(scan(doc), "## Wanted") {
		got = append(got, l.n)
	}
	want := []int{3, 4, 5, 6, 7, 8, 12}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("section lines = %v, want %v", got, want)
	}
	if s := sectionLines(scan(doc), "## Missing"); s != nil {
		t.Fatalf("missing section = %v", s)
	}
}

func TestEndsInComment(t *testing.T) {
	cases := []struct {
		text      string
		inComment bool
		want      bool
	}{
		{"plain", false, false},
		{"<!-- open", false, true},
		{"<!-- closed -->", false, false},
		{"still inside", true, true},
		{"closes --> after", true, false},
		{"closes --> then <!-- opens", true, true},
		{"<!--> empty", false, false},
		{"<!---> empty", false, false},
		{"`<!--` in code", false, false},
		{"``a ` <!-- b`` in code", false, false},
		{"` unmatched <!--", false, true},
		{"`a`` <!-- b` a longer run doesn't close the span", false, false},
	}
	for _, c := range cases {
		if got := endsInComment(c.text, c.inComment); got != c.want {
			t.Errorf("endsInComment(%q, %v) = %v, want %v", c.text, c.inComment, got, c.want)
		}
	}
}

func TestScanComments(t *testing.T) {
	doc := "a\n<!--\n```\n-->\n```\nx\n```"
	var comment, fence []int
	for _, l := range scan(doc) {
		if l.inComment {
			comment = append(comment, l.n)
		}
		if l.inFence {
			fence = append(fence, l.n)
		}
	}
	if !reflect.DeepEqual(comment, []int{3, 4}) || !reflect.DeepEqual(fence, []int{5, 6, 7}) {
		t.Fatalf("comment = %v, fence = %v", comment, fence)
	}
}
