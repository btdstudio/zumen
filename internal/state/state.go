// Package state configures kiso's phase machine for zumen and migrates older
// STATE.json files.
//
// The planning loop is intake → explore → requirements → design → plan. Every
// phase is gated. Explore is skippable: `zumen change new` skips it unless the
// intake points at a real codebase.
//
// Layout:
//
//	STATE.json         phase machine: { schema, changes } maps each change folder
//	                   to its own { phase, status, gates, sources, updatedAt }
//	                   record — the single store, committed so approvals survive
//	                   a clone. LoadState resolves the active record.
//	LOG.md             one timestamped line per state change / gate (committed)
//	.zumen/active      the active change's name; gitignored, because each
//	                   checkout works on its own change
//	REPOS.json         logical repos (committed); .zumen/paths.json is local
//	specs/             per-change folders: <id>-<slug>/requirements.md,
//	                   design.md, tasks.json
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	kiso "github.com/btdstudio/kiso/state"
	"github.com/btdstudio/kiso/workspace"
)

// Machine is the planning loop.
var Machine = kiso.Machine{Tool: "zumen", Phases: []kiso.Phase{
	{Name: "intake", Kind: kiso.Gated},
	{Name: "explore", Kind: kiso.Gated, Skippable: true},
	{Name: "requirements", Kind: kiso.Gated},
	{Name: "design", Kind: kiso.Gated},
	{Name: "plan", Kind: kiso.Gated},
}}

// NoRepoReason explains the explore skip that `zumen change new` records.
const NoRepoReason = "no repo in the intake"

// Now is the injectable clock used for timestamps (updatedAt and log lines).
// Tests override it to make output deterministic.
var Now = time.Now

var store = kiso.Store{
	Machine:   Machine,
	SpecsDir:  "specs",
	Commit:    true,
	Now:       func() time.Time { return Now() },
	Migrate:   migrate,
	Preflight: refuseOldLocation,
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
// the active change. Without a repo there's nothing to explore, so explore starts
// skipped. Sources are committed with it, so a clone has what the record points
// at.
func SeedChange(root, name string, hasRepo bool, sources []string, repos ...string) (State, error) {
	opts := kiso.SeedOptions{Sources: sources, Repos: repos, Paths: sources}
	if !hasRepo {
		opts.Skip = map[string]string{"explore": NoRepoReason}
	}
	return store.Seed(root, name, opts)
}

// AdvanceGate sets a gate on the active change, logs it with any notes, and
// commits.
func AdvanceGate(root, phase, status, reason string, notes ...string) (State, error) {
	return store.Advance(root, phase, status, reason, notes...)
}

// Reopen moves the active change back to a phase, logs it and commits.
func Reopen(root, phase, reason string) (State, error) { return store.Reopen(root, phase, reason) }

// CloseChange closes a change by name, logs it and commits.
func CloseChange(root, name, reason string) (State, error) {
	return store.CloseChange(root, name, reason)
}

// AppendLog appends one timestamped line to LOG.md.
func AppendLog(root, line string) error { return store.Log(root, line) }

// migrate upgrades a pre-schema STATE.json, whose changes carried a `mode`. A
// change that wasn't "existing" never ran explore, so explore becomes skipped; an
// "existing" change still has to finish it. Approvals carry over as they are.
func migrate(data []byte) ([]byte, error) {
	var old struct {
		Changes map[string]map[string]any `json:"changes"`
	}
	if err := json.Unmarshal(data, &old); err != nil {
		return nil, err
	}
	for _, rec := range old.Changes {
		if rec == nil {
			continue
		}
		mode, _ := rec["mode"].(string)
		delete(rec, "mode")
		if mode == "existing" {
			continue
		}
		gates, _ := rec["gates"].(map[string]any)
		if gates == nil {
			gates = map[string]any{}
			rec["gates"] = gates
		}
		if explore, _ := gates["explore"].(string); explore == "" {
			gates["explore"] = kiso.Skipped
		}
	}
	return json.Marshal(map[string]any{"schema": kiso.Schema, "changes": old.Changes})
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
