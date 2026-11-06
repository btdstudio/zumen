// Package state owns the zumen workspace contract: the STATE.json phase machine,
// the LOG.md changeset, the local active-change pointer, and the top-level
// specs/ dir.
//
// The planning loop is intake → explore? → requirements → design → plan. Every
// phase is gated; "explore" runs only when the active change's mode is
// "existing" (the intake points at a real codebase).
//
// Layout:
//
//	STATE.json         phase machine: { changes } maps each change folder to
//	                   its own { phase, status, mode, gates, sources, updatedAt }
//	                   record — the single store, committed so approvals survive
//	                   a clone. LoadState resolves the active record.
//	LOG.md             one timestamped line per state change / gate (committed)
//	.zumen/active      the active change's name; gitignored, because each
//	                   checkout works on its own change
//	specs/             per-change folders: <id>-<slug>/requirements.md,
//	                   design.md, tasks.json
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PHASES is the gated planning loop in order. Explore is skipped for
// greenfield changes but is always present in the machine.
var PHASES = []string{"intake", "explore", "requirements", "design", "plan"}

// GATE_STATES is the per-phase gate status. "" = not started.
var GATE_STATES = []string{"", "draft", "await-approval", "approved"}

// MODES is the set of change modes. "existing" means the intake points at a
// real codebase, so the loop runs explore before requirements.
var MODES = []string{"greenfield", "existing"}

// Now is the injectable clock used for timestamps (updatedAt and log lines).
// Tests override it to make output deterministic.
var Now = time.Now

func isoMillis(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

// Gates is the per-phase gate map. Field order matches PHASES order so
// marshaling is stable.
type Gates struct {
	Intake       string `json:"intake"`
	Explore      string `json:"explore"`
	Requirements string `json:"requirements"`
	Design       string `json:"design"`
	Plan         string `json:"plan"`
}

// State is the resolved active machine (what LoadState returns).
type State struct {
	Phase     string
	Status    string
	Mode      string
	Gates     Gates
	Sources   []string
	Change    string
	UpdatedAt string
}

// changeRecord is the per-change phase-machine record stored in state.json.
type changeRecord struct {
	Phase   string   `json:"phase"`
	Status  string   `json:"status"`
	Mode    string   `json:"mode"`
	Gates   Gates    `json:"gates"`
	Sources []string `json:"sources"`
	// UpdatedAt lives per change so branches touching different changes merge
	// cleanly.
	UpdatedAt string `json:"updatedAt"`
}

// stateFile is the on-disk STATE.json shape.
type stateFile struct {
	Changes map[string]*changeRecord `json:"changes"`
}

func Workspace(root string) string  { return filepath.Join(root, ".zumen") }
func StatePath(root string) string  { return filepath.Join(root, "STATE.json") }
func ActivePath(root string) string { return filepath.Join(Workspace(root), "active") }
func SpecsDir(root string) string   { return filepath.Join(root, "specs") }
func LogPath(root string) string    { return filepath.Join(root, "LOG.md") }

// ChangeDir returns the path to one change folder. An empty change resolves to
// the specs base itself (a guard for ad-hoc outputs when nothing is active).
func ChangeDir(root, change string) string {
	if change != "" {
		return filepath.Join(SpecsDir(root), change)
	}
	return SpecsDir(root)
}

// ArtifactPath is an artifact inside a specific change folder.
func ArtifactPath(root, change, name string) string {
	return filepath.Join(ChangeDir(root, change), name)
}

var changeIDRe = regexp.MustCompile(`^(\d+)-`)

// changeNameRe is the strict grammar for an accepted change folder name: a
// numeric id, a hyphen, then the kebab-case slug Slugify produces. SwitchTo
// requires this so a name can never escape specs/ via `..` or a separator.
var changeNameRe = regexp.MustCompile(`^(\d+)-[a-z0-9]+(?:-[a-z0-9]+)*$`)

// NextChangeId is the next sequential change id: highest numeric `<id>-` prefix
// under specs + 1.
func NextChangeId(root string) int {
	entries, err := os.ReadDir(SpecsDir(root))
	if err != nil {
		return 1
	}
	max := 0
	for _, e := range entries {
		if m := changeIDRe.FindStringSubmatch(e.Name()); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > max {
				max = n
			}
		}
	}
	return max + 1
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify lowercases to a short kebab-case slug so change folders read as
// <id>-<slug>.
func Slugify(text string) string {
	s := strings.ToLower(text)
	s = slugNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	r := []rune(s)
	if len(r) > 40 {
		s = string(r[:40])
	}
	return s
}

func DefaultState() State {
	return State{
		Mode:    "greenfield",
		Gates:   Gates{},
		Sources: []string{},
	}
}

func normalizeSources(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func normalizeMode(m string) string {
	for _, v := range MODES {
		if v == m {
			return m
		}
	}
	return "greenfield"
}

// ReadActive returns the active change's name ("" when none is set).
func ReadActive(root string) (string, error) {
	p := ActivePath(root)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("zumen: cannot read %s: %v", p, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func writeActive(root, name string) error {
	if err := os.MkdirAll(Workspace(root), 0o755); err != nil {
		return err
	}
	return os.WriteFile(ActivePath(root), []byte(name+"\n"), 0o644)
}

// resolve builds the machine for one change from its record (nil = fresh).
func resolve(name string, rec *changeRecord) State {
	st := DefaultState()
	st.Change = name
	if rec != nil {
		if rec.Phase != "" {
			st.Phase = rec.Phase
		}
		if rec.Status != "" {
			st.Status = rec.Status
		}
		st.Mode = normalizeMode(rec.Mode)
		st.Gates = rec.Gates
		st.UpdatedAt = rec.UpdatedAt
		if rec.Sources != nil {
			st.Sources = append([]string{}, rec.Sources...)
		}
	}
	return st
}

// LoadState resolves the active machine. It returns DefaultState when nothing
// is recorded, and errors when STATE.json cannot be parsed.
func LoadState(root string) (State, error) {
	changes, err := ReadChanges(root)
	if err != nil {
		return State{}, err
	}
	name, err := ReadActive(root)
	if err != nil {
		return State{}, err
	}
	return resolve(name, changes[name]), nil
}

// SaveState stamps updatedAt, writes the change's record to STATE.json
// (preserving every other change's), and makes it the active change.
func SaveState(root string, st *State) error {
	st.UpdatedAt = isoMillis(Now())
	st.Mode = normalizeMode(st.Mode)
	if err := os.MkdirAll(Workspace(root), 0o755); err != nil {
		return err
	}
	changes, err := ReadChanges(root)
	if err != nil {
		return err
	}
	changes[st.Change] = &changeRecord{
		Phase:     st.Phase,
		Status:    st.Status,
		Mode:      st.Mode,
		Gates:     st.Gates,
		Sources:   normalizeSources(st.Sources),
		UpdatedAt: st.UpdatedAt,
	}
	buf, err := json.MarshalIndent(stateFile{Changes: changes}, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	if err := os.WriteFile(StatePath(root), buf, 0o644); err != nil {
		return err
	}
	return writeActive(root, st.Change)
}

// ReadChanges returns the per-change phase-machine records in STATE.json (empty
// when absent). An unreadable or unparseable file is an error, never "no
// changes": SaveState writes this map back, so treating it as empty would wipe
// every recorded change.
func ReadChanges(root string) (map[string]*changeRecord, error) {
	p := StatePath(root)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			legacy := filepath.Join(Workspace(root), "state.json")
			if _, statErr := os.Stat(legacy); statErr == nil {
				return nil, fmt.Errorf("zumen: found old %s — move it and .zumen/log.md to STATE.json and LOG.md at the workspace root, then run `zumen switch <change>`", legacy)
			}
			return map[string]*changeRecord{}, nil
		}
		return nil, fmt.Errorf("zumen: cannot read %s: %v", p, err)
	}
	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("zumen: cannot parse %s: %v", p, err)
	}
	if sf.Changes == nil {
		return map[string]*changeRecord{}, nil
	}
	return sf.Changes, nil
}

// KnownChange is one change folder annotated with any recorded progress.
type KnownChange struct {
	Name   string `json:"name"`
	ID     int    `json:"id"`
	Mode   string `json:"mode"`
	Phase  string `json:"phase"`
	Status string `json:"status"`
	Gates  Gates  `json:"gates"`
}

// KnownChanges lists the numbered folders under specs/ sorted by id.
func KnownChanges(root string) ([]KnownChange, error) {
	changes, err := ReadChanges(root)
	if err != nil {
		return nil, err
	}
	out := []KnownChange{}
	entries, err := os.ReadDir(SpecsDir(root))
	if err != nil {
		return out, nil
	}
	for _, e := range entries {
		name := e.Name()
		m := changeIDRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		id, _ := strconv.Atoi(m[1])
		rec := changes[name]
		kc := KnownChange{Name: name, ID: id, Mode: "greenfield", Gates: Gates{}}
		if rec != nil {
			kc.Phase = rec.Phase
			kc.Status = rec.Status
			kc.Mode = normalizeMode(rec.Mode)
			kc.Gates = rec.Gates
		}
		out = append(out, kc)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].ID > out[j].ID; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out, nil
}

// SwitchTo points the workspace at a different change folder and resumes its
// phase machine. The target folder must exist under specs/ and match the
// strict change-name grammar (no `..`, no separators, no symlinks). Switching to
// a recorded change only moves the local pointer, leaving STATE.json untouched.
func SwitchTo(root, name string) (State, error) {
	if !changeNameRe.MatchString(name) {
		return State{}, fmt.Errorf(`zumen: unknown change "%s"`, name)
	}
	dir := ChangeDir(root, name)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return State{}, fmt.Errorf(`zumen: unknown change "%s"`, name)
	}
	changes, err := ReadChanges(root)
	if err != nil {
		return State{}, err
	}
	st := resolve(name, changes[name])
	if changes[name] != nil {
		if err := writeActive(root, name); err != nil {
			return State{}, err
		}
		return st, nil
	}
	if err := SaveState(root, &st); err != nil {
		return State{}, err
	}
	return st, nil
}

// SetGate advances the phase machine: starts the given phase and sets its
// status. Every zumen phase is gated; statuses come from GATE_STATES. Validates
// names only; CheckGate is the loop-order rule.
func SetGate(st *State, phase, status string) error {
	found := false
	for _, p := range PHASES {
		if p == phase {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown phase: %s", phase)
	}
	for _, s := range GATE_STATES {
		if s == status {
			st.Phase = phase
			st.Status = status
			switch phase {
			case "intake":
				st.Gates.Intake = status
			case "explore":
				st.Gates.Explore = status
			case "requirements":
				st.Gates.Requirements = status
			case "design":
				st.Gates.Design = status
			case "plan":
				st.Gates.Plan = status
			}
			return nil
		}
	}
	return fmt.Errorf("unknown gate status: %s", status)
}

// gateStatus returns one phase's gate status.
func gateStatus(g Gates, phase string) string {
	switch phase {
	case "intake":
		return g.Intake
	case "explore":
		return g.Explore
	case "requirements":
		return g.Requirements
	case "design":
		return g.Design
	case "plan":
		return g.Plan
	}
	return ""
}

// CheckGate reports whether setting phase to status respects the loop: every
// earlier phase must be approved, and explore only exists in existing mode.
// Clearing a gate (status "") is always allowed. It returns the violation, or
// nil; whether a violation blocks is the caller's policy (settings.Gates).
func CheckGate(st State, phase, status string) error {
	if status == "" {
		return nil
	}
	if phase == "explore" && st.Mode != "existing" {
		return fmt.Errorf("zumen: explore does not apply to %s changes", st.Mode)
	}
	for _, p := range PHASES {
		if p == phase {
			return nil
		}
		if p == "explore" && st.Mode != "existing" {
			continue
		}
		if got := gateStatus(st.Gates, p); got != "approved" {
			return fmt.Errorf("zumen: %s requires %s to be approved (currently %q)", phase, p, got)
		}
	}
	return nil
}

// SeedChange creates a new change folder and records its initial machine
// (phase intake, status draft) with the given mode and sources, making it the
// active change.
func SeedChange(root, name, mode string, sources []string) (State, error) {
	if !changeNameRe.MatchString(name) {
		return State{}, fmt.Errorf("zumen: invalid change name %q", name)
	}
	if err := os.MkdirAll(ChangeDir(root, name), 0o755); err != nil {
		return State{}, err
	}
	// Git drops empty folders, and a clone needs this one for `zumen switch`.
	if err := os.WriteFile(filepath.Join(ChangeDir(root, name), ".gitkeep"), nil, 0o644); err != nil {
		return State{}, err
	}
	st := DefaultState()
	st.Change = name
	st.Mode = normalizeMode(mode)
	st.Sources = normalizeSources(sources)
	if err := SetGate(&st, "intake", "draft"); err != nil {
		return State{}, err
	}
	if err := SaveState(root, &st); err != nil {
		return State{}, err
	}
	return st, nil
}

// AdvanceGate loads the active machine, sets the given phase/status, saves, and
// appends a log line. It errors when no change is active.
func AdvanceGate(root, phase, status string) (State, error) {
	st, err := LoadState(root)
	if err != nil {
		return State{}, err
	}
	if st.Change == "" {
		return State{}, fmt.Errorf("zumen: no active change — run `zumen switch <change>` first")
	}
	// The pointer is local, so a branch switch can leave it naming a change that
	// doesn't exist here.
	if _, err := os.Stat(ChangeDir(root, st.Change)); err != nil {
		return State{}, fmt.Errorf("zumen: active change %q has no folder under specs/ — run `zumen switch <change>`", st.Change)
	}
	if err := SetGate(&st, phase, status); err != nil {
		return State{}, err
	}
	if err := SaveState(root, &st); err != nil {
		return State{}, err
	}
	if err := AppendLog(root, st.Change+": "+phase+" → "+status); err != nil {
		return State{}, err
	}
	return st, nil
}

// AppendLog appends one timestamped line to LOG.md.
func AppendLog(root, line string) error {
	if err := os.MkdirAll(Workspace(root), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(LogPath(root), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "- %s %s\n", isoMillis(Now()), line)
	return err
}
