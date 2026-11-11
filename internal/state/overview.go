package state

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/btdstudio/kiso/repos"
	kiso "github.com/btdstudio/kiso/state"
	"github.com/btdstudio/zumen/internal/overview"
	"github.com/btdstudio/zumen/internal/plan"
	"github.com/btdstudio/zumen/internal/settings"
)

// Refresh is set here, not in the store literal: RefreshOverview reads state
// through store, which Go would reject as an initialization cycle. storeAt
// copies store at call time, so every transition carries it.
func init() {
	store.Refresh = func(root string, c kiso.Change) error {
		_, err := RefreshOverview(root, c.Name)
		return err
	}
}

// OverviewPath is a change's README, relative to the workspace root.
func OverviewPath(change string) string { return "specs/" + change + "/README.md" }

// OverviewError reports a problem with the README at rel as
// "zumen: <rel>: <problem>", dropping a "zumen: " prefix the problem already
// carries. It unwraps to err.
func OverviewError(rel string, err error) error { return &overviewError{rel, err} }

type overviewError struct {
	rel string
	err error
}

func (e *overviewError) Error() string {
	return "zumen: " + e.rel + ": " + strings.TrimPrefix(e.err.Error(), "zumen: ")
}

func (e *overviewError) Unwrap() error { return e.err }

// Overview builds a change's README: the merged file, the file on disk (nil
// when there's none), and its root-relative path. It doesn't check that the
// change exists. Errors aren't wrapped with the path.
func Overview(root, change string) (merged, existing []byte, rel string, err error) {
	rel = OverviewPath(change)
	m, err := plan.Load(root, change)
	if err != nil {
		return nil, nil, rel, err
	}
	changes, err := ReadChanges(root)
	if err != nil {
		return nil, nil, rel, err
	}
	var gates, reasons map[string]string
	if c := changes[change]; c != nil {
		gates, reasons = c.Gates, c.Reasons
	}
	var phases []string
	for _, p := range Machine.Phases {
		phases = append(phases, p.Name)
	}
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		return nil, nil, rel, err
	}
	reg, err := repos.Load(root, "zumen")
	if err != nil {
		return nil, nil, rel, err
	}
	existing, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, rel, err
	}
	base := existing
	if existing == nil {
		base = overview.Stub(m.Title)
	}
	narrative, err := overview.Narrative(base)
	if err != nil {
		return nil, nil, rel, err
	}
	region := overview.Render(overview.Input{
		Model: m, Phases: phases, Gates: gates, Reasons: reasons,
		Repos: reg.Names(), ADRDir: loaded.Effective.AdrDir,
		NarrativeEmpty: overview.NarrativeEmpty(narrative),
	})
	merged, err = overview.Merge(base, region)
	if err != nil {
		return nil, nil, rel, err
	}
	return merged, existing, rel, nil
}

// RefreshOverview regenerates the named changes' READMEs under `overview:
// auto`, writing each only when its bytes change, and returns the paths it
// wrote. Under `manual` it does nothing. A change that fails is left alone and
// reported in err as "zumen: <path>: <problem>"; the others are still written.
func RefreshOverview(root string, changes ...string) (written []string, err error) {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		return nil, err
	}
	if loaded.Effective.Overview != settings.OverviewAuto {
		return nil, nil
	}
	seen := map[string]bool{}
	var names []string
	for _, c := range changes {
		if c != "" && !seen[c] {
			seen[c] = true
			names = append(names, c)
		}
	}
	sort.Strings(names)
	var errs []error
	for _, c := range names {
		merged, existing, rel, err := Overview(root, c)
		if err != nil {
			errs = append(errs, OverviewError(rel, err))
			continue
		}
		if existing != nil && bytes.Equal(existing, merged) {
			continue
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), merged, 0o644); err != nil {
			errs = append(errs, OverviewError(rel, err))
			continue
		}
		written = append(written, rel)
	}
	return written, errors.Join(errs...)
}
