package icm

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/btdstudio/zumen"
)

// scaf is a small scaffold: A, B and C are zumen's, Tech is the user's.
const scaf = "# T\n\nintro\n\n## A\n\na body\n\n## B\n\nb body\n\n## Tech stack & standards\n\n- <FILL IN>\n\n## C\n\nc body\n"

func TestMergeAgents(t *testing.T) {
	for _, tc := range []struct {
		name, user, want string
	}{
		{
			"keeps the user's sections, title and intro; updates zumen's",
			"# My project\n\nMy own intro.\n\n## A\n\nold a\n\n## Team notes\n\nnotes\n\n## B\n\nold b\n\n## Tech stack & standards\n\n- Go 1.22\n\n## C\n\nold c\n",
			"# My project\n\nMy own intro.\n\n## A\n\na body\n\n## Team notes\n\nnotes\n\n## B\n\nb body\n\n## Tech stack & standards\n\n- Go 1.22\n\n## C\n\nc body\n",
		},
		{
			"a ## line inside a fence isn't a heading, in zumen's section or the user's",
			"# P\n\n## A\n\nold a\n\n## B\n\nold b\n\n```\n## not a heading\n```\n\n## Tech stack & standards\n\nx\n\n```\n## A\n```\n\n~~~\n## A\n~~~\n\n````\n```\n## C\n````\n\n## C\n\nold c\n",
			"# P\n\n## A\n\na body\n\n## B\n\nb body\n\n## Tech stack & standards\n\nx\n\n```\n## A\n```\n\n~~~\n## A\n~~~\n\n````\n```\n## C\n````\n\n## C\n\nc body\n",
		},
		{
			"a ### subsection belongs to its section",
			"# P\n\n## A\n\nold a\n\n### sub\n\nmore\n\n## B\n\nold b\n\n## Tech stack & standards\n\nx\n\n## C\n\nold c\n",
			"# P\n\n## A\n\na body\n\n## B\n\nb body\n\n## Tech stack & standards\n\nx\n\n## C\n\nc body\n",
		},
		{
			"reordered sections keep the user's order",
			"# P\n\n## C\n\nold c\n\n## Tech stack & standards\n\nx\n\n## B\n\nold b\n\n## A\n\nold a\n",
			"# P\n\n## C\n\nc body\n\n## Tech stack & standards\n\nx\n\n## B\n\nb body\n\n## A\n\na body\n",
		},
		{
			"a missing section goes after its preceding scaffold section",
			"# P\n\n## A\n\nold a\n\n## Tech stack & standards\n\nx\n\n## C\n\nold c\n",
			"# P\n\n## A\n\na body\n\n## B\n\nb body\n\n## Tech stack & standards\n\nx\n\n## C\n\nc body\n",
		},
		{
			"a missing first section goes right after the intro",
			"# P\n\nintro\n\n## B\n\nold b\n\n## Tech stack & standards\n\nx\n\n## C\n\nold c\n",
			"# P\n\nintro\n\n## A\n\na body\n\n## B\n\nb body\n\n## Tech stack & standards\n\nx\n\n## C\n\nc body\n",
		},
		{
			"a missing first section after an intro with no trailing blank line",
			"# P\n## B\n\nold b\n\n## Tech stack & standards\n\nx\n\n## C\n\nold c\n",
			"# P\n\n## A\n\na body\n\n## B\n\nb body\n\n## Tech stack & standards\n\nx\n\n## C\n\nc body\n",
		},
		{
			"a missing last section after a section with no trail, and no final newline",
			"# P\n\n## A\n\nold a\n\n## B\n\nold b\n\n## Tech stack & standards\n\nx",
			"# P\n\n## A\n\na body\n\n## B\n\nb body\n\n## Tech stack & standards\n\nx\n\n## C\n\nc body",
		},
		{
			"a missing section inserted before a user section keeps a blank line",
			"# P\n\n## A\n\nold a\n\n## B\n\nold b\n\n## Tech stack & standards\n\nx\n\n## Team notes\n\nn\n",
			"# P\n\n## A\n\na body\n\n## B\n\nb body\n\n## Tech stack & standards\n\nx\n\n## C\n\nc body\n\n## Team notes\n\nn\n",
		},
		{
			"a deleted user-owned section isn't put back",
			"# P\n\n## A\n\nold a\n\n## B\n\nold b\n\n## C\n\nold c\n",
			"# P\n\n## A\n\na body\n\n## B\n\nb body\n\n## C\n\nc body\n",
		},
		{
			"no final newline is kept",
			"# P\n\n## A\n\nold a\n\n## B\n\nold b\n\n## Tech stack & standards\n\nx\n\n## C\n\nold c",
			"# P\n\n## A\n\na body\n\n## B\n\nb body\n\n## Tech stack & standards\n\nx\n\n## C\n\nc body",
		},
		{
			"the user's spacing between sections is kept",
			"# P\n\n## A\n\nold a\n\n\n\n## B\n\nold b\n## Tech stack & standards\n\nx\n\n## C\n\nold c\n",
			"# P\n\n## A\n\na body\n\n\n\n## B\n\nb body\n## Tech stack & standards\n\nx\n\n## C\n\nc body\n",
		},
		{
			"an up-to-date file is unchanged",
			scaf,
			scaf,
		},
	} {
		got, reason := MergeAgents(tc.user, scaf)
		if reason != "" {
			t.Errorf("%s: refused: %s", tc.name, reason)
			continue
		}
		if got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
		if again, _ := MergeAgents(got, scaf); again != got {
			t.Errorf("%s: not idempotent:\n first %q\nsecond %q", tc.name, got, again)
		}
	}
}

func TestMergeAgentsRefuses(t *testing.T) {
	for _, tc := range []struct{ name, user, reason string }{
		{"no ## headings", "mine", "no ## headings"},
		{"only a title", "# P\n\njust text\n", "no ## headings"},
		{"empty", "", "no ## headings"},
		{"headings only in a fence", "# P\n\n```\n## A\n```\n", "no ## headings"},
		{"a duplicated zumen heading", "# P\n\n## A\n\none\n\n## B\n\nb\n\n## A\n\ntwo\n", `duplicated heading "A"`},
	} {
		got, reason := MergeAgents(tc.user, scaf)
		if got != "" || reason != tc.reason {
			t.Errorf("%s: got (%q, %q), want a refusal with %q", tc.name, got, reason, tc.reason)
		}
	}
	// A duplicated user-owned or user's own heading is fine.
	user := "# P\n\n## Team notes\n\n1\n\n## Team notes\n\n2\n\n## Tech stack & standards\n\nx\n\n## Tech stack & standards\n\ny\n"
	if got, reason := MergeAgents(user, scaf); reason != "" || !strings.Contains(got, "\n1\n") || !strings.Contains(got, "\n2\n") || !strings.Contains(got, "\ny\n") {
		t.Errorf("duplicate user headings: got (%q, %q), want a merge that keeps both", got, reason)
	}
}

func TestMergeAgentsDropsRetiredSections(t *testing.T) {
	user := "# P\n\n## Old\n\nstale\n\n## A\n\nold a\n\n## B\n\nold b\n\n## Tech stack & standards\n\nx\n\n## C\n\nold c\n"
	got, reason := mergeAgents(user, scaf, userOwnedSections, []string{"Old"})
	want := "# P\n\n## A\n\na body\n\n## B\n\nb body\n\n## Tech stack & standards\n\nx\n\n## C\n\nc body\n"
	if reason != "" || got != want {
		t.Fatalf("got (%q, %q), want %q", got, reason, want)
	}
	if again, _ := mergeAgents(got, scaf, userOwnedSections, []string{"Old"}); again != got {
		t.Fatalf("not idempotent: %q", again)
	}
}

func scaffoldAgents(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(zumen.ScaffoldFS, "scaffold/AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The real scaffold: a user who edited Tech stack, added a section and let the
// routing table go stale gets zumen's sections back, and nothing else changes.
func TestMergeAgentsRealScaffold(t *testing.T) {
	real := scaffoldAgents(t)
	doc := parseAgents(real)
	var user []string
	user = append(user, strings.Join(doc.preamble, "\n")+"\n")
	for _, sec := range doc.sections {
		text := strings.Join(sec.body, "\n") + "\n\n"
		switch sec.heading {
		case "Tech stack & standards":
			text = "## Tech stack & standards\n\n- Go 1.22, Postgres\n\n"
		case "Routing table":
			text = "## Routing table\n\nstale\n\n"
		}
		user = append(user, text)
	}
	user = append(user, "## Team notes\n\nkeep me\n")
	got, reason := MergeAgents(strings.Join(user, ""), real)
	if reason != "" {
		t.Fatal(reason)
	}
	merged := parseAgents(got)
	want := map[string]agentsSection{}
	for _, sec := range doc.sections {
		want[sec.heading] = sec
	}
	for _, sec := range merged.sections {
		switch sec.heading {
		case "Tech stack & standards":
			if !strings.Contains(strings.Join(sec.body, "\n"), "Go 1.22, Postgres") {
				t.Errorf("Tech stack was overwritten: %q", sec.body)
			}
		case "Team notes":
			if !strings.Contains(strings.Join(sec.body, "\n"), "keep me") {
				t.Errorf("Team notes was overwritten: %q", sec.body)
			}
		default:
			if strings.Join(sec.body, "\n") != strings.Join(want[sec.heading].body, "\n") {
				t.Errorf("%s doesn't match the scaffold:\n%s", sec.heading, strings.Join(sec.body, "\n"))
			}
		}
	}
	if len(merged.sections) != len(doc.sections)+1 {
		t.Errorf("sections = %d, want %d", len(merged.sections), len(doc.sections)+1)
	}
	if again, _ := MergeAgents(got, real); again != got {
		t.Error("the merge of the real scaffold isn't idempotent")
	}
}

// The merge needs each zumen heading to appear once in the scaffold.
func TestScaffoldAgentsHeadingsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, sec := range parseAgents(scaffoldAgents(t)).sections {
		if seen[sec.heading] {
			t.Errorf("scaffold AGENTS.md repeats the heading %q", sec.heading)
		}
		seen[sec.heading] = true
	}
	if len(seen) == 0 {
		t.Fatal("no ## headings in the scaffold's AGENTS.md")
	}
	for _, name := range userOwnedSections {
		if !seen[name] {
			t.Errorf("user-owned section %q isn't in the scaffold", name)
		}
	}
}
