// Package state configures kiso's phase machine for zumen and migrates older
// STATE.json files.
//
// The planning loop is intake → research → explore → synthesis → spec →
// architecture → breakdown → publish. Research, explore and synthesis are
// skippable, and publish is a run phase.
//
// Layout:
//
//	STATE.json         phase machine: { schema, changes } maps each change folder
//	                   to its own { phase, status, gates, sources, repos, updatedAt }
//	                   record — the single store, committed so approvals survive
//	                   a clone. LoadState resolves the active record.
//	LOG.md             one timestamped line per state change / gate (committed)
//	.zumen/active      the active change's name; gitignored, because each
//	                   checkout works on its own change
//	REPOS.json         logical repos (committed); .zumen/paths.json is local
//	specs/             per-change folders: <id>-<slug>/functional-spec.md,
//	                   architecture.md, tickets/<ID>.md
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	kiso "github.com/btdstudio/kiso/state"
	"github.com/btdstudio/kiso/workspace"
	"github.com/btdstudio/zumen/internal/plan"
	"github.com/btdstudio/zumen/internal/settings"
)

// Machine is the planning loop. Check is plan.CheckArtifacts: it's set here,
// in the literal, because the store copies Machine by value. kiso's Advance
// never runs it; `zumen gate` does, through CheckArtifacts, and applies the
// gates policy to what it reports.
var Machine = kiso.Machine{Tool: "zumen", Check: plan.CheckArtifacts, Phases: []kiso.Phase{
	{Name: "intake", Kind: kiso.Gated},
	{Name: "research", Kind: kiso.Gated, Skippable: true},
	{Name: "explore", Kind: kiso.Gated, Skippable: true},
	{Name: "synthesis", Kind: kiso.Gated, Skippable: true},
	{Name: "spec", Kind: kiso.Gated},
	{Name: "architecture", Kind: kiso.Gated},
	{Name: "breakdown", Kind: kiso.Gated},
	{Name: "publish", Kind: kiso.Run},
}}

// Schema is zumen's STATE.json version. Version 2 is the v2 loop.
const Schema = 2

// Reasons for the skips `zumen change new` records.
const (
	NoRepoReason     = "no repo in the intake"
	SingleRepoReason = "fewer than two repos in scope"
)

// Now is the injectable clock used for timestamps (updatedAt and log lines).
// Tests override it to make output deterministic.
var Now = time.Now

var store = kiso.Store{
	Machine:  Machine,
	SpecsDir: "specs",
	// Gates commit each change's research notes and findings too.
	ChangeDirs: []string{"01_research", "02_explore"},
	Schema:     Schema,
	Commit:     true,
	Now:        func() time.Time { return Now() },
	Migrate:    migrate,
	Preflight:  refuseOldLocation,
}

// storeAt is the store for the workspace at root, carrying its "vcs" setting so
// a local workspace (vcs: none) never runs git. Only the wrappers that record a
// transition need it; paths and reads use store directly.
func storeAt(root string) (kiso.Store, error) {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		return kiso.Store{}, err
	}
	s := store
	s.VCS = loaded.VCS
	return s, nil
}

type (
	// State is one change's machine.
	State = kiso.Change
	// KnownChange is one change folder annotated with any recorded progress.
	KnownChange = kiso.KnownChange
)

func Workspace(root string) string  { return workspace.Dir(root, "zumen") }
func StatePath(root string) string  { return store.StatePath(root) }
func ActivePath(root string) string { return workspace.ActivePath(root, "zumen") }
func SpecsDir(root string) string   { return store.SpecsPath(root) }
func LogPath(root string) string    { return store.LogPath(root) }

// ChangeDir returns the path to one change folder. An empty change resolves to
// the specs base itself (a guard for ad-hoc outputs when nothing is active).
func ChangeDir(root, change string) string { return store.ChangeDir(root, change) }

// ArtifactPath is an artifact inside a specific change folder.
func ArtifactPath(root, change, name string) string {
	return filepath.Join(ChangeDir(root, change), name)
}

// NextChangeId is the next sequential change id.
func NextChangeId(root string) int { return store.NextID(root) }

// Slugify lowercases to a short kebab-case slug.
func Slugify(text string) string { return kiso.Slugify(text) }

// DefaultState is the machine when nothing is recorded.
func DefaultState() State { return kiso.NewChange("") }

// ReadActive returns the active change's name ("" when none is set).
func ReadActive(root string) (string, error) { return workspace.ReadActive(root, "zumen") }

// LoadState resolves the active machine.
func LoadState(root string) (State, error) { return store.Load(root) }

// SaveState writes the change's record and makes it the active change.
func SaveState(root string, st *State) error { return store.Save(root, st) }

// ReadChanges returns the recorded changes.
func ReadChanges(root string) (map[string]*State, error) { return store.ReadChanges(root) }

// ChangeGates returns a copy of the named change's gates. A change with no record,
// or an empty name, has no gates yet, so it gets an empty map.
func ChangeGates(root, name string) (map[string]string, error) {
	changes, err := ReadChanges(root)
	if err != nil {
		return nil, err
	}
	gates := map[string]string{}
	if c, ok := changes[name]; ok && c != nil {
		for phase, status := range c.Gates {
			gates[phase] = status
		}
	}
	return gates, nil
}

// KnownChanges lists the numbered folders under specs/ sorted by id.
func KnownChanges(root string) ([]KnownChange, error) { return store.Known(root) }

// SwitchTo points the workspace at a different change folder.
func SwitchTo(root, name string) (State, error) { return store.Switch(root, name) }

// SetGate starts the given phase and sets its status.
func SetGate(st *State, phase, status, reason string) error {
	return Machine.SetGate(st, phase, status, reason)
}

// CheckGate reports whether setting phase to status respects the loop.
func CheckGate(st State, phase, status string) error { return Machine.CheckGate(st, phase, status) }

// SeedChange creates a change folder and records its initial machine, making it
// the active change. Without a repo there's nothing to explore, and with fewer than
// two there's nothing to synthesize, so those start skipped. Under vcs: git,
// sources are committed with it, so a clone has what the record points at.
func SeedChange(root, name string, hasRepo bool, sources []string, repos ...string) (State, error) {
	opts := kiso.SeedOptions{Sources: sources, Repos: repos, Paths: sources}
	opts.Skip = map[string]string{}
	if !hasRepo {
		opts.Skip["explore"] = NoRepoReason
	}
	if len(repos) < 2 {
		opts.Skip["synthesis"] = SingleRepoReason
	}
	s, err := storeAt(root)
	if err != nil {
		return State{}, err
	}
	return s.Seed(root, name, opts)
}

// AdvanceGate sets a gate on the active change, logs it with any notes, and
// commits under vcs: git.
func AdvanceGate(root, phase, status, reason string, notes ...string) (State, error) {
	s, err := storeAt(root)
	if err != nil {
		return State{}, err
	}
	return s.Advance(root, phase, status, reason, notes...)
}

// Reopen moves the active change back to a phase, logs it and commits under
// vcs: git.
func Reopen(root, phase, reason string) (State, error) {
	s, err := storeAt(root)
	if err != nil {
		return State{}, err
	}
	return s.Reopen(root, phase, reason)
}

// CloseChange closes a change by name, logs it and commits under vcs: git.
func CloseChange(root, name, reason string) (State, error) {
	s, err := storeAt(root)
	if err != nil {
		return State{}, err
	}
	return s.CloseChange(root, name, reason)
}

// Record logs lines and, under vcs: git, commits them with the given paths,
// relative to root.
func Record(root string, lines []string, paths ...string) error {
	s, err := storeAt(root)
	if err != nil {
		return err
	}
	return s.Record(root, lines, paths...)
}

// AppendLog appends one timestamped line to LOG.md.
func AppendLog(root, line string) error { return store.Log(root, line) }

// renamedPhases maps v1 phases to the phases that replaced them.
var renamedPhases = map[string]string{"requirements": "spec", "design": "architecture", "plan": "breakdown"}

// migrate upgrades an older STATE.json, keeping every approval. Before schema 1, a
// change carried a `mode`: one that wasn't "existing" never ran explore, so explore
// becomes skipped. For schema 2, v1's phases are renamed. Research and synthesis
// didn't exist, so a change already past them has them skipped.
func migrate(data []byte) ([]byte, error) {
	var old struct {
		Schema  int                       `json:"schema"`
		Changes map[string]map[string]any `json:"changes"`
	}
	if err := json.Unmarshal(data, &old); err != nil {
		return nil, err
	}
	for _, rec := range old.Changes {
		if rec == nil {
			continue
		}
		gates := map[string]any{}
		if raw, ok := rec["gates"].(map[string]any); ok {
			for phase, status := range raw {
				if s, _ := status.(string); s != "" {
					if renamed, ok := renamedPhases[phase]; ok {
						phase = renamed
					}
					gates[phase] = s
				}
			}
		}
		rec["gates"] = gates
		if phase, _ := rec["phase"].(string); renamedPhases[phase] != "" {
			rec["phase"] = renamedPhases[phase]
		}
		if mode, _ := rec["mode"].(string); old.Schema == 0 && mode != "existing" && gates["explore"] == nil {
			gates["explore"] = kiso.Skipped
		}
		delete(rec, "mode")
		pastSynthesis := false
		for _, phase := range []string{"spec", "architecture", "breakdown"} {
			pastSynthesis = pastSynthesis || gates[phase] != nil
		}
		if pastSynthesis || (gates["explore"] != nil && gates["explore"] != kiso.Skipped) {
			if gates["research"] == nil {
				gates["research"] = kiso.Skipped
			}
		}
		repos, _ := rec["repos"].([]any)
		if (pastSynthesis || len(repos) < 2) && gates["synthesis"] == nil {
			gates["synthesis"] = kiso.Skipped
		}
	}
	return json.Marshal(map[string]any{"schema": Schema, "changes": old.Changes})
}

// refuseOldLocation stops a workspace from an earlier version, which kept its
// state under .zumen/, from silently looking empty.
func refuseOldLocation(root string) error {
	if _, err := os.Stat(filepath.Join(root, "STATE.json")); err == nil {
		return nil
	}
	legacy := filepath.Join(Workspace(root), "state.json")
	if _, err := os.Stat(legacy); err == nil {
		return fmt.Errorf("zumen: found old %s — move it and .zumen/log.md to STATE.json and LOG.md at the workspace root, then run `zumen switch <change>`", legacy)
	}
	return nil
}
