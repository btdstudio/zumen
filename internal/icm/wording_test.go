package icm

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/btdstudio/zumen"
)

var (
	commitWord = regexp.MustCompile(`(?i)\bcommit`)
	listStart  = regexp.MustCompile(`^(\s*[-*]|\s*\d+\.)\s`)
)

// mdBlocks splits markdown into the units a reader takes as one statement: a list
// item with its indented continuation lines, a paragraph, a heading, or a single
// line inside a code fence. Each block carries the heading it sits under.
func mdBlocks(text string) (blocks, headings []string) {
	var cur []string
	heading := ""
	inFence := false
	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, strings.Join(cur, " "))
			headings = append(headings, heading)
			cur = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			flush()
			inFence = !inFence
		case inFence:
			cur = []string{line}
			flush()
		case trimmed == "":
			flush()
		case strings.HasPrefix(trimmed, "#"):
			flush()
			heading = trimmed
		case listStart.MatchString(line):
			flush()
			cur = []string{line}
		default:
			cur = append(cur, line)
		}
	}
	flush()
	return blocks, headings
}

// AC12: every scaffolded statement about committing is qualified by vcs: git, so
// a local workspace's instructions never promise commits.
func TestScaffoldCommitWordingIsConditional(t *testing.T) {
	sub, err := fs.Sub(zumen.ScaffoldFS, "scaffold")
	if err != nil {
		t.Fatal(err)
	}
	err = fs.WalkDir(sub, ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(rel, ".md") {
			return err
		}
		data, err := fs.ReadFile(sub, rel)
		if err != nil {
			return err
		}
		blocks, headings := mdBlocks(string(data))
		for i, block := range blocks {
			if !commitWord.MatchString(block) {
				continue
			}
			if headings[i] == "## Version control" || strings.Contains(block, "vcs: git") {
				continue
			}
			t.Errorf("%s: commit wording not qualified by `vcs: git`: %s", rel, strings.TrimSpace(block))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
