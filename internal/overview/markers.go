package overview

import (
	"fmt"
	"regexp"
	"strings"
)

// The four region markers (ADR-0004). Each sits on its own line and is matched by
// prefix, so the wording after the prefix can change.
const (
	narrativeStart = "<!-- zumen:narrative"
	narrativeEnd   = "<!-- /zumen:narrative"
	generatedStart = "<!-- zumen:generated"
	generatedEnd   = "<!-- /zumen:generated"
)

var markerOrder = []string{narrativeStart, narrativeEnd, generatedStart, generatedEnd}

// Stub is a new overview file: the title, an empty narrative with a prompt, and
// an empty generated region.
func Stub(title string) []byte {
	return []byte("# " + title + "\n\n" +
		narrativeStart + " -->\n" +
		"<!-- About 250 words: the problem, the shape of the solution, who's affected, and what's out of scope. -->\n" +
		narrativeEnd + " -->\n\n" +
		generatedStart + " — do not edit; run `zumen overview` -->\n" +
		generatedEnd + " -->\n")
}

// lines splits text into lines that keep their endings.
func lines(text []byte) []string {
	return strings.SplitAfter(string(text), "\n")
}

// markers finds the line index of each marker, checking that each appears
// exactly once and in order.
func markers(ls []string) (map[string]int, error) {
	at := map[string]int{}
	for i, l := range ls {
		for _, m := range markerOrder {
			if !strings.HasPrefix(l, m) {
				continue
			}
			if prev, dup := at[m]; dup {
				return nil, fmt.Errorf("duplicated marker %s --> (lines %d and %d)", m, prev+1, i+1)
			}
			at[m] = i
		}
	}
	for _, m := range markerOrder {
		if _, ok := at[m]; !ok {
			return nil, fmt.Errorf("missing marker %s -->", m)
		}
	}
	for i := 1; i < len(markerOrder); i++ {
		a, b := markerOrder[i-1], markerOrder[i]
		if at[b] < at[a] {
			return nil, fmt.Errorf("markers out of order: %s --> (line %d) comes before %s --> (line %d)", b, at[b]+1, a, at[a]+1)
		}
	}
	return at, nil
}

// Merge replaces the lines between the generated markers with region, keeping
// every other byte. Missing, duplicated or out-of-order markers are an error.
func Merge(existing []byte, region string) ([]byte, error) {
	ls := lines(existing)
	at, err := markers(ls)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for _, l := range ls[:at[generatedStart]+1] {
		b.WriteString(l)
	}
	b.WriteString(region)
	for _, l := range ls[at[generatedEnd]:] {
		b.WriteString(l)
	}
	return []byte(b.String()), nil
}

// Narrative returns the text between the narrative markers.
func Narrative(existing []byte) (string, error) {
	ls := lines(existing)
	at, err := markers(ls)
	if err != nil {
		return "", err
	}
	return strings.Join(ls[at[narrativeStart]+1:at[narrativeEnd]], ""), nil
}

var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)

// NarrativeEmpty reports whether a narrative is only whitespace once HTML
// comments, such as the stub's prompt, are removed.
func NarrativeEmpty(narrative string) bool {
	return strings.TrimSpace(htmlComment.ReplaceAllString(narrative, "")) == ""
}
