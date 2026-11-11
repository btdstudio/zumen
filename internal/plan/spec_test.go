package plan

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// AC1: the synthetic fixture mirrors a real spec's layout.
func TestParseSpecFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	reqs, findings := ParseSpec(string(data))
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	var want []string
	for i := 1; i <= 23; i++ {
		want = append(want, fmt.Sprintf("R%d", i))
	}
	for i := 1; i <= 6; i++ {
		want = append(want, fmt.Sprintf("N%d", i))
	}
	var got []string
	for _, r := range reqs {
		got = append(got, r.ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v\nwant  %v", got, want)
	}
	priority := map[string]string{"R3": "should", "R4": "could", "R8": "should", "R14": "should", "R19": "should", "R22": "should"}
	lines := strings.Split(string(data), "\n")
	for _, r := range reqs {
		p := priority[r.ID]
		if p == "" {
			p = "must"
		}
		if r.Priority != p {
			t.Errorf("%s priority = %q, want %q", r.ID, r.Priority, p)
		}
		if r.Kind != r.ID[0] {
			t.Errorf("%s kind = %c", r.ID, r.Kind)
		}
		if !regexp.MustCompile(`^[-*] ` + r.ID + `\. `).MatchString(lines[r.Line-1]) {
			t.Errorf("%s line %d = %q", r.ID, r.Line, lines[r.Line-1])
		}
	}
}

func TestParseSpec(t *testing.T) {
	type req struct {
		id, priority string
		line         int
	}
	head := "# Spec — X\n\n## Functional requirements\n\n" // requirements start on line 5
	cases := []struct {
		name     string
		body     string
		reqs     []req
		findings []Finding
	}{
		{
			name: "template line is not a requirement (AC2)",
			body: "- R1. <capability, verifiable> — **must | should | could**\n",
		},
		{
			name: "ellipsis template line is not a requirement",
			body: "- R1. … — **must**\n",
		},
		{
			name:     "missing priority warns (AC3)",
			body:     "- R4. A thing with no priority\n",
			reqs:     []req{{"R4", "", 5}},
			findings: []Finding{{Warning, "R4", "line 5: R4 has no priority (expected — **must**, — **should** or — **could**)"}},
		},
		{
			name:     "the template's priority alternatives warn",
			body:     "- R1. A thing. — **must | should | could**\n",
			reqs:     []req{{"R1", "", 5}},
			findings: []Finding{{Warning, "R1", "line 5: R1 has no priority (expected — **must**, — **should** or — **could**)"}},
		},
		{
			name:     "priority needs the em dash and a space",
			body:     "- R1. A thing. —**must**\n- R2. A thing. - **must**\n",
			reqs:     []req{{"R1", "", 5}, {"R2", "", 6}},
			findings: []Finding{{Warning, "R1", "line 5: R1 has no priority (expected — **must**, — **should** or — **could**)"}, {Warning, "R2", "line 6: R2 has no priority (expected — **must**, — **should** or — **could**)"}},
		},
		{
			name: "first priority wins, text after is allowed",
			body: "- R1. A thing. — **could** (later — **must**)\n",
			reqs: []req{{"R1", "could", 5}},
		},
		{
			name:     "duplicate id is an error naming both lines (AC4)",
			body:     "- R2. One. — **must**\n- R3. Other. — **must**\n- R2. Again. — **should**\n",
			reqs:     []req{{"R2", "must", 5}, {"R3", "must", 6}},
			findings: []Finding{{Error, "R2", "line 7: R2 is already defined on line 5"}},
		},
		{
			name: "three occurrences give two errors",
			body: "- R2. One. — **must**\n- R2. Two. — **must**\n- R2. Three. — **must**\n",
			reqs: []req{{"R2", "must", 5}},
			findings: []Finding{
				{Error, "R2", "line 6: R2 is already defined on line 5"},
				{Error, "R2", "line 7: R2 is already defined on line 5"},
			},
		},
		{
			name: "fenced lines are ignored (AC5)",
			body: "```\n- R1. Fenced. — **must**\n```\n~~~md\n- R2. Fenced. — **must**\n~~~\n- R3. Real. — **must**\n",
			reqs: []req{{"R3", "must", 11}},
		},
		{
			name: "an unclosed fence hides the rest",
			body: "- R1. Real. — **must**\n```\n- R2. Fenced. — **must**\n",
			reqs: []req{{"R1", "must", 5}},
		},
		{
			name: "other sections are ignored (AC5)",
			body: "- R1. Real. — **must**\n## Acceptance criteria\n- R2. Not here. — **must**\n",
			reqs: []req{{"R1", "must", 5}},
		},
		{
			name: "level-3 headings stay inside, an H1 ends it",
			body: "### Functional\n- R1. One. — **must**\n### Non-functional\n- N1. Two. — **should**\n# Appendix\n- R2. Not here. — **must**\n",
			reqs: []req{{"R1", "must", 6}, {"N1", "should", 8}},
		},
		{
			name: "only list items with an id and a dot count",
			body: "R1. Not a bullet. — **must**\n- R2 no dot — **must**\n- AC1. Not an id. — **must**\n- r3. Lower case. — **must**\n  * R4. Indented star. — **must**\n",
			reqs: []req{{"R4", "must", 9}},
		},
		{
			name: "a placeholder in inline code doesn't hide the line",
			body: "- R1. Returns `List<T>`. — **must**\n",
			reqs: []req{{"R1", "must", 5}},
		},
		{
			name: "crlf",
			body: "- R1. One. — **must**\r\n- N1. Two. — **could**\r\n",
			reqs: []req{{"R1", "must", 5}, {"N1", "could", 6}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reqs, findings := ParseSpec(head + c.body)
			var got []req
			for _, r := range reqs {
				got = append(got, req{r.ID, r.Priority, r.Line})
				if r.Kind != r.ID[0] {
					t.Errorf("%s kind = %c", r.ID, r.Kind)
				}
			}
			if !reflect.DeepEqual(got, c.reqs) {
				t.Errorf("reqs = %+v, want %+v", got, c.reqs)
			}
			if !reflect.DeepEqual(findings, c.findings) {
				t.Errorf("findings = %+v, want %+v", findings, c.findings)
			}
		})
	}
}

func TestParseSpecNoSection(t *testing.T) {
	reqs, findings := ParseSpec("# Spec\n\n- R1. Outside. — **must**\n")
	if reqs != nil || findings != nil {
		t.Fatalf("reqs = %v, findings = %v", reqs, findings)
	}
}

func TestSeverityString(t *testing.T) {
	if Error.String() != "error" || Warning.String() != "warning" {
		t.Fatalf("got %q, %q", Error, Warning)
	}
}
