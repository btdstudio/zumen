package plan

import "strings"

// line is one line of a markdown document. n is 1-based; inFence is true for
// a fence's opening and closing lines and every line between them. open and
// close mark those two lines, and info is an opening line's info string.
// inComment is true for a line that starts inside an HTML comment.
type line struct {
	n         int
	text      string
	inFence   bool
	inComment bool
	open      bool
	close     bool
	info      string
}

// prose reports whether a line is neither fenced code nor inside an HTML
// comment, so its text counts.
func (l line) prose() bool { return !l.inFence && !l.inComment }

// scan splits markdown into lines and marks fenced code and HTML comments,
// with the same rules as kiso's placeholder.Find, so the two agree on what's
// hidden. A fence opens with any indentation and three or more backticks or
// tildes (a backtick fence's info string can't contain a backtick), and
// closes with at least as many of the same character and nothing else. A
// fence can't open inside a comment. Fences and comments left unclosed run to
// the end of the document.
func scan(markdown string) []line {
	raw := strings.Split(markdown, "\n")
	out := make([]line, 0, len(raw))
	var fenceChar byte
	fenceLen := 0
	inComment := false
	for i, text := range raw {
		text = strings.TrimSuffix(text, "\r")
		l := line{n: i + 1, text: text, inComment: inComment}
		if fenceLen > 0 {
			l.inFence = true
			if c, n, rest, ok := fenceMarker(text); ok && c == fenceChar && n >= fenceLen && strings.TrimSpace(rest) == "" {
				l.close = true
				fenceLen = 0
			}
		} else if c, n, rest, ok := fenceMarker(text); ok && !inComment && (c == '~' || !strings.Contains(rest, "`")) {
			l.inFence, l.open, l.info = true, true, strings.TrimSpace(rest)
			fenceChar, fenceLen = c, n
		} else {
			inComment = endsInComment(text, inComment)
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

// endsInComment reports whether a line ends inside an HTML comment, given
// whether it starts inside one. Like kiso's placeholder.Find, it reads left to
// right, and an inline code span hides a "<!--" that starts inside it.
func endsInComment(text string, inComment bool) bool {
	i := 0
	for i < len(text) {
		if inComment {
			j := strings.Index(text[i:], "-->")
			if j < 0 {
				return true
			}
			i += j + 3
			inComment = false
			continue
		}
		switch {
		case text[i] == '`':
			n := 0
			for i+n < len(text) && text[i+n] == '`' {
				n++
			}
			if end := closingTicks(text, i+n, n); end >= 0 {
				i = end
			} else {
				i += n
			}
		case strings.HasPrefix(text[i:], "<!-->"):
			i += 5
		case strings.HasPrefix(text[i:], "<!--->"):
			i += 6
		case strings.HasPrefix(text[i:], "<!--"):
			i += 4
			inComment = true
		default:
			i++
		}
	}
	return inComment
}

// closingTicks returns the index just past the first run of exactly n
// backticks at or after from, or -1 if there's none.
func closingTicks(text string, from, n int) int {
	for i := from; i < len(text); {
		if text[i] != '`' {
			i++
			continue
		}
		m := 0
		for i+m < len(text) && text[i+m] == '`' {
			m++
		}
		if m == n {
			return i + m
		}
		i += m
	}
	return -1
}

// endsSection reports whether a line outside fences and comments is a level-1 or level-2
// heading, which ends a section.
func endsSection(l line) bool {
	return l.prose() && (strings.HasPrefix(l.text, "# ") || strings.HasPrefix(l.text, "## "))
}

// isHeading reports whether l is exactly the given heading, outside fences
// and comments.
// Trailing spaces are allowed.
func isHeading(l line, heading string) bool {
	return l.prose() && strings.TrimRight(l.text, " \t") == heading
}

// sectionLines returns the lines of every section headed by heading, up to the
// next `# ` or `## ` heading outside fences and comments. The heading lines themselves
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
