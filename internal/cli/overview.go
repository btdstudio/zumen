package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/btdstudio/kiso/repos"
	"github.com/btdstudio/zumen/internal/overview"
	"github.com/btdstudio/zumen/internal/plan"
	"github.com/btdstudio/zumen/internal/settings"
	"github.com/btdstudio/zumen/internal/state"
)

const overviewUsage = "usage: zumen overview [<change>] [--check] [--stdout]"

// runOverview writes or checks specs/<change>/README.md. Exit codes: 0 written
// or current; 1 an error, or --check found it stale; 2 --check couldn't check.
func runOverview(root string, args []string) int {
	var change string
	var check, stdout bool
	for _, a := range args {
		switch {
		case a == "--check":
			check = true
		case a == "--stdout":
			stdout = true
		case strings.HasPrefix(a, "-") || change != "":
			fmt.Fprintln(os.Stderr, overviewUsage)
			return 1
		default:
			change = a
		}
	}
	if check && stdout {
		fmt.Fprintln(os.Stderr, overviewUsage)
		return 1
	}
	failed := 1
	if check {
		failed = 2
	}
	merged, existing, rel, err := buildOverview(root, change)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return failed
	}
	switch {
	case stdout:
		os.Stdout.Write(merged)
		return 0
	case check:
		if existing != nil && bytes.Equal(existing, merged) {
			fmt.Printf("%s is up to date\n", rel)
			return 0
		}
		fmt.Printf("%s is out of date — run zumen overview\n", rel)
		return 1
	}
	if existing != nil && bytes.Equal(existing, merged) {
		fmt.Printf("%s is up to date\n", rel)
		return 0
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), merged, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("wrote %s\n", rel)
	return 0
}

// buildOverview renders a change's README. existing is the file on disk, or
// nil when there's none; rel is its root-relative path.
func buildOverview(root, change string) (merged, existing []byte, rel string, err error) {
	if change == "" {
		st, err := state.LoadState(root)
		if err != nil {
			return nil, nil, "", err
		}
		if st.Name == "" {
			return nil, nil, "", errors.New("zumen: no active change — name one, or run `zumen switch <change>` first")
		}
		change = st.Name
	}
	known, err := state.KnownChanges(root)
	if err != nil {
		return nil, nil, "", err
	}
	found := false
	for _, k := range known {
		found = found || k.Name == change
	}
	if !found {
		return nil, nil, "", fmt.Errorf("zumen: no change named %q", change)
	}
	rel = "specs/" + change + "/README.md"

	m, err := plan.Load(root, change)
	if err != nil {
		return nil, nil, "", err
	}
	changes, err := state.ReadChanges(root)
	if err != nil {
		return nil, nil, "", err
	}
	var gates, reasons map[string]string
	if c := changes[change]; c != nil {
		gates, reasons = c.Gates, c.Reasons
	}
	var phases []string
	for _, p := range state.Machine.Phases {
		phases = append(phases, p.Name)
	}
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		return nil, nil, "", err
	}
	reg, err := repos.Load(root, "zumen")
	if err != nil {
		return nil, nil, "", err
	}

	existing, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, "", err
	}
	base := existing
	if existing == nil {
		base = overview.Stub(m.Title)
	}
	narrative, err := overview.Narrative(base)
	if err != nil {
		return nil, nil, "", fmt.Errorf("zumen: %s: %w", rel, err)
	}
	region := overview.Render(overview.Input{
		Model: m, Phases: phases, Gates: gates, Reasons: reasons,
		Repos: reg.Names(), ADRDir: loaded.Effective.AdrDir,
		NarrativeEmpty: overview.NarrativeEmpty(narrative),
	})
	merged, err = overview.Merge(base, region)
	if err != nil {
		return nil, nil, "", fmt.Errorf("zumen: %s: %w", rel, err)
	}
	return merged, existing, rel, nil
}
