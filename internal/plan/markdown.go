package plan

import "strings"

// line is one line of a markdown document. n is 1-based; inFence is true for
// a fence's opening and closing lines and every line between them. open and
// close mark those two lines, and info is an opening line's info string.
type line struct {
	n       int
	text    string
	inFence bool
	open    bool
	close   bool
	info    string
}

// scan splits markdown into lines and marks fenced code, with the same rules
// as kiso's placeholder.Find, so the two agree on what's code. A fence opens
// with any indentation and three or more backticks or tildes (a backtick
// fence's info string can't contain a backtick), and closes with at least as
// many of the same character and nothing else. An unclosed fence runs to the
// end of the document.
func scan(markdown string) []line {
	raw := strings.Split(markdown, "\n")
	out := make([]line, 0, len(raw))
	var fenceChar byte
	fenceLen := 0
	for i, text := range raw {
		text = strings.TrimSuffix(text, "\r")
		l := line{n: i + 1, text: text}
		if fenceLen > 0 {
			l.inFence = true
			if c, n, rest, ok := fenceMarker(text); ok && c == fenceChar && n >= fenceLen && strings.TrimSpace(rest) == "" {
				l.close = true
				fenceLen = 0
			}
		} else if c, n, rest, ok := fenceMarker(text); ok && (c == '~' || !strings.Contains(rest, "`")) {
			l.inFence, l.open, l.info = true, true, strings.TrimSpace(rest)
			fenceChar, fenceLen = c, n
		}
		out = append(out, l)
	}
	return out
}

// fenceMarker reports whether text starts a fence line: any indentation, then
// a run of three or more backticks or tildes. rest is what follows the run.
func fenceMarker(text string) (c byte, n int, rest string, ok bool) {
	s := strings.TrimLeft(text, " \t")
	if s == "" || (s[0] != '`' && s[0] != '~') {
		return 0, 0, "", false
	}
	c = s[0]
	for n < len(s) && s[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0, "", false
	}
	return c, n, s[n:], true
}

// endsSection reports whether a line outside a fence is a level-1 or level-2
// heading, which ends a section.
func endsSection(l line) bool {
	return !l.inFence && (strings.HasPrefix(l.text, "# ") || strings.HasPrefix(l.text, "## "))
}

// isHeading reports whether l is exactly the given heading, outside a fence.
// Trailing spaces are allowed.
func isHeading(l line, heading string) bool {
	return !l.inFence && strings.TrimRight(l.text, " \t") == heading
}

// sectionLines returns the lines of every section headed by heading, up to the
// next `# ` or `## ` heading outside a fence. The heading lines themselves
// aren't included.
func sectionLines(lines []line, heading string) []line {
	var out []line
	in := false
	for _, l := range lines {
		if isHeading(l, heading) {
			in = true
			continue
		}
		if endsSection(l) {
			in = false
			continue
		}
		if in {
			out = append(out, l)
		}
	}
	return out
}
