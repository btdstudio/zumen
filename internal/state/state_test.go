package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fixedNow(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestLoadStateReturnsDefaultWhenAbsent(t *testing.T) {
	st, err := LoadState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(st, DefaultState()) {
		t.Fatalf("got %+v, want default", st)
	}
}

func TestSaveThenLoadRoundtripsAndStampsUpdatedAt(t *testing.T) {
	root := t.TempDir()
	st := DefaultState()
	if err := SetGate(&st, "requirements", "await-approval"); err != nil {
		t.Fatal(err)
	}
	st.Sources = []string{"00_intake/1.md"}
	wantGates := st.Gates
	if err := SaveState(root, &st); err != nil {
		t.Fatal(err)
	}
	if st.UpdatedAt == "" {
		t.Fatal("updatedAt must be stamped")
	}
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Phase != "requirements" || loaded.Status != "await-approval" {
		t.Fatalf("phase/status = %q/%q", loaded.Phase, loaded.Status)
	}
	if !reflect.DeepEqual(loaded.Sources, []string{"00_intake/1.md"}) {
		t.Fatalf("sources = %v", loaded.Sources)
	}
	if !reflect.DeepEqual(loaded.Gates, wantGates) {
		t.Fatalf("gates = %+v, want %+v", loaded.Gates, wantGates)
	}
}

func TestSetGateUpdatesPhaseStatusAndMap(t *testing.T) {
	st := DefaultState()
	if err := SetGate(&st, "design", "draft"); err != nil {
		t.Fatal(err)
	}
	if st.Phase != "design" || st.Status != "draft" || st.Gates.Design != "draft" {
		t.Fatalf("phase/status/gates.design = %q/%q/%q", st.Phase, st.Status, st.Gates.Design)
	}
	if st.Gates.Intake != "" || st.Gates.Explore != "" || st.Gates.Requirements != "" || st.Gates.Plan != "" {
		t.Fatalf("unrelated gates should stay empty, got %+v", st.Gates)
	}
}

func TestSetGateRejectsUnknownPhaseAndStatus(t *testing.T) {
	st := DefaultState()
	if err := SetGate(&st, "bogus", "draft"); err == nil || !strings.Contains(err.Error(), "unknown phase") {
		t.Fatalf("want unknown phase error, got %v", err)
	}
	if err := SetGate(&st, "plan", "nope"); err == nil || !strings.Contains(err.Error(), "unknown gate status") {
		t.Fatalf("want unknown gate status error, got %v", err)
	}
}

func TestDefaultStateSeedsEveryPhase(t *testing.T) {
	if !reflect.DeepEqual(PHASES, []string{"intake", "explore", "requirements", "design", "plan"}) {
		t.Fatalf("PHASES = %v", PHASES)
	}
	if !reflect.DeepEqual(MODES, []string{"greenfield", "existing"}) {
		t.Fatalf("MODES = %v", MODES)
	}
	g := DefaultState().Gates
	if g.Intake != "" || g.Explore != "" || g.Requirements != "" || g.Design != "" || g.Plan != "" {
		t.Fatalf("gates should be all empty, got %+v", g)
	}
	if DefaultState().Mode != "greenfield" {
		t.Fatalf("default mode = %q", DefaultState().Mode)
	}
}

func TestModeRoundTripsAndNormalizes(t *testing.T) {
	root := t.TempDir()
	st := DefaultState()
	st.Change = "1-x"
	st.Mode = "existing"
	if err := SetGate(&st, "explore", "draft"); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, &st); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Mode != "existing" || loaded.Gates.Explore != "draft" {
		t.Fatalf("loaded = %+v", loaded)
	}
	if normalizeMode("bogus") != "greenfield" {
		t.Fatalf("normalizeMode(bogus) should be greenfield")
	}
	if normalizeMode("existing") != "existing" {
		t.Fatalf("normalizeMode(existing) should be existing")
	}
}

func TestSaveStateWritesSlimShapeAndLoadResolves(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-alpha"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root), "2-beta"), 0o755)

	a := DefaultState()
	a.Change = "1-alpha"
	a.Mode = "greenfield"
	if err := SetGate(&a, "requirements", "approved"); err != nil {
		t.Fatal(err)
	}
	a.Sources = []string{"00_intake/1.md"}
	if err := SaveState(root, &a); err != nil {
		t.Fatal(err)
	}

	b, err := SwitchTo(root, "2-beta")
	if err != nil {
		t.Fatal(err)
	}
	b.Mode = "existing"
	if err := SetGate(&b, "design", "draft"); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, &b); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(StatePath(root))
	if err != nil {
		t.Fatal(err)
	}
	var sf stateFile
	if err := json.Unmarshal(raw, &sf); err != nil {
		t.Fatal(err)
	}
	// The only top-level key is changes; the active change is not shared.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	if _, ok := top["changes"]; !ok || len(top) != 1 {
		t.Fatalf("top-level keys wrong: %s", raw)
	}
	if name, err := ReadActive(root); err != nil || name != "2-beta" {
		t.Fatalf("active = %q, %v", name, err)
	}
	if sf.Changes["2-beta"].UpdatedAt == "" {
		t.Fatal("per-change updatedAt must be stamped")
	}
	if sf.Changes["1-alpha"].Gates.Requirements != "approved" {
		t.Fatalf("1-alpha requirements = %q", sf.Changes["1-alpha"].Gates.Requirements)
	}
	if sf.Changes["2-beta"].Gates.Design != "draft" {
		t.Fatalf("2-beta design = %q", sf.Changes["2-beta"].Gates.Design)
	}
	if sf.Changes["2-beta"].Mode != "existing" {
		t.Fatalf("2-beta mode = %q", sf.Changes["2-beta"].Mode)
	}

	active, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if active.Change != "2-beta" || active.Phase != "design" || active.Status != "draft" || active.Mode != "existing" {
		t.Fatalf("active = %+v", active)
	}

	switched, err := SwitchTo(root, "1-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if switched.Phase != "requirements" || switched.Status != "approved" {
		t.Fatalf("switched phase/status = %q/%q", switched.Phase, switched.Status)
	}
	if switched.Mode != "greenfield" {
		t.Fatalf("switched mode = %q", switched.Mode)
	}
	if !reflect.DeepEqual(switched.Sources, []string{"00_intake/1.md"}) {
		t.Fatalf("switched sources = %v", switched.Sources)
	}
}

func TestArtifactPaths(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "specs", "1-auth-flow", "requirements.md")
	if got := ArtifactPath(root, "1-auth-flow", "requirements.md"); got != want {
		t.Fatalf("artifactPath = %q, want %q", got, want)
	}
	if got := ArtifactPath(root, "", "findings.md"); got != filepath.Join(root, "specs", "findings.md") {
		t.Fatalf("artifactPath empty change = %q", got)
	}
}

func TestNextChangeId(t *testing.T) {
	root := t.TempDir()
	if NextChangeId(root) != 1 {
		t.Fatalf("no specs dir -> 1, got %d", NextChangeId(root))
	}
	os.MkdirAll(SpecsDir(root), 0o755)
	if NextChangeId(root) != 1 {
		t.Fatalf("empty specs -> 1")
	}
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-auth-flow"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root), "2-refactor"), 0o755)
	os.WriteFile(filepath.Join(SpecsDir(root), "README.md"), []byte("x"), 0o644)
	if NextChangeId(root) != 3 {
		t.Fatalf("got %d, want 3", NextChangeId(root))
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Auth Flow!":        "auth-flow",
		"  ":                "",
		"Refactor DB layer": "refactor-db-layer",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Fatalf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
	if len(Slugify("a "+strings.Repeat("x", 100))) > 40 {
		t.Fatalf("slugify too long")
	}
}

func TestChangeDirResolvesToSpecsBaseWhenEmpty(t *testing.T) {
	root := t.TempDir()
	if ChangeDir(root, "") != SpecsDir(root) {
		t.Fatalf("empty change should resolve to specs base")
	}
	if ChangeDir(root, "3-thing") != filepath.Join(SpecsDir(root), "3-thing") {
		t.Fatalf("named change path wrong")
	}
}

func TestEachChangeKeepsOwnMachine(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-auth-flow"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root), "2-refactor"), 0o755)

	st := DefaultState()
	st.Change = "1-auth-flow"
	if err := SetGate(&st, "plan", "approved"); err != nil {
		t.Fatal(err)
	}
	st.Sources = []string{"00_intake/1.md"}
	if err := SaveState(root, &st); err != nil {
		t.Fatal(err)
	}

	other, err := SwitchTo(root, "2-refactor")
	if err != nil {
		t.Fatal(err)
	}
	if other.Change != "2-refactor" || other.Phase != "" || other.Status != "" {
		t.Fatalf("fresh folder = %+v", other)
	}

	back, err := SwitchTo(root, "1-auth-flow")
	if err != nil {
		t.Fatal(err)
	}
	if back.Change != "1-auth-flow" || back.Phase != "plan" || back.Status != "approved" {
		t.Fatalf("back = %+v", back)
	}
	if !reflect.DeepEqual(back.Sources, []string{"00_intake/1.md"}) {
		t.Fatalf("back sources = %v", back.Sources)
	}
	if back.Gates.Plan != "approved" {
		t.Fatalf("back gates.plan = %q", back.Gates.Plan)
	}
}

func TestSwitchRejectsUnknownsAndKnownChangesLists(t *testing.T) {
	root := t.TempDir()
	if _, err := SwitchTo(root, "nope"); err == nil || !strings.Contains(err.Error(), "unknown change") {
		t.Fatalf("want unknown change (nope), got %v", err)
	}
	if _, err := SwitchTo(root, ""); err == nil || !strings.Contains(err.Error(), "unknown change") {
		t.Fatalf("want unknown change (empty), got %v", err)
	}
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-a"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root), "2-b"), 0o755)
	os.WriteFile(filepath.Join(SpecsDir(root), "README.md"), []byte("x"), 0o644)

	kc, err := KnownChanges(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range kc {
		names = append(names, c.Name)
	}
	if !reflect.DeepEqual(names, []string{"1-a", "2-b"}) {
		t.Fatalf("knownChanges names = %v", names)
	}

	st := DefaultState()
	st.Change = "1-a"
	if err := SetGate(&st, "requirements", "await-approval"); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, &st); err != nil {
		t.Fatal(err)
	}

	kc, _ = KnownChanges(root)
	find := func(name string) KnownChange {
		for _, c := range kc {
			if c.Name == name {
				return c
			}
		}
		return KnownChange{}
	}
	if find("1-a").Status != "await-approval" {
		t.Fatalf("1-a status = %q", find("1-a").Status)
	}
	if find("2-b").Status != "" {
		t.Fatalf("2-b status = %q", find("2-b").Status)
	}
}

func TestAppendLog(t *testing.T) {
	root := t.TempDir()
	if err := AppendLog(root, "requirements → await-approval"); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(root, "requirements approved"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(LogPath(root))
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "- 20") || !strings.Contains(lines[0], "T") {
		t.Fatalf("line 0 missing timestamp: %q", lines[0])
	}
	if !strings.Contains(lines[0], "await-approval") || !strings.Contains(lines[1], "approved") {
		t.Fatalf("lines wrong: %v", lines)
	}
}

func TestUpdatedAtFormatMatchesJSISOString(t *testing.T) {
	orig := Now
	defer func() { Now = orig }()
	Now = fixedNow(time.Date(2026, 10, 24, 9, 41, 59, 123000000, time.UTC))
	st := DefaultState()
	st.Change = "1-x"
	if err := SaveState(t.TempDir(), &st); err != nil {
		t.Fatal(err)
	}
	if st.UpdatedAt != "2026-10-24T09:41:59.123Z" {
		t.Fatalf("updatedAt = %q", st.UpdatedAt)
	}
}

func TestSwitchRejectsTraversalNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"1-../escape", "1-a/../../x", "1-Weird_Name", "nope", ""} {
		if _, err := SwitchTo(root, name); err == nil || !strings.Contains(err.Error(), "unknown change") {
			t.Fatalf("switch(%q) should be rejected, got %v", name, err)
		}
	}
}

func TestSeedChangeCreatesAndActivates(t *testing.T) {
	root := t.TempDir()
	st, err := SeedChange(root, "1-draft-posts", "existing", []string{"00_intake/1.md"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Change != "1-draft-posts" || st.Mode != "existing" || st.Phase != "intake" || st.Status != "draft" {
		t.Fatalf("st = %+v", st)
	}
	if st.Gates.Intake != "draft" {
		t.Fatalf("gates.intake = %q", st.Gates.Intake)
	}
	if _, err := os.Stat(filepath.Join(SpecsDir(root), "1-draft-posts")); err != nil {
		t.Fatalf("change folder not created: %v", err)
	}
	active, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if active.Change != "1-draft-posts" {
		t.Fatalf("active = %+v", active)
	}
}

func TestSeedChangeRejectsBadName(t *testing.T) {
	if _, err := SeedChange(t.TempDir(), "../evil", "greenfield", nil); err == nil || !strings.Contains(err.Error(), "invalid change name") {
		t.Fatalf("want invalid change name, got %v", err)
	}
}

func TestAdvanceGateTransitionsAndLogs(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedChange(root, "1-x", "greenfield", nil); err != nil {
		t.Fatal(err)
	}
	st, err := AdvanceGate(root, "requirements", "await-approval")
	if err != nil {
		t.Fatal(err)
	}
	if st.Phase != "requirements" || st.Status != "await-approval" || st.Gates.Requirements != "await-approval" {
		t.Fatalf("st = %+v", st)
	}
	b, _ := os.ReadFile(LogPath(root))
	if !strings.Contains(string(b), "requirements → await-approval") {
		t.Fatalf("log missing transition: %q", b)
	}
}

func TestAdvanceGateRequiresActiveChange(t *testing.T) {
	if _, err := AdvanceGate(t.TempDir(), "requirements", "approved"); err == nil || !strings.Contains(err.Error(), "no active change") {
		t.Fatalf("want no active change, got %v", err)
	}
}

func TestCheckGate(t *testing.T) {
	approved := Gates{Intake: "approved", Explore: "approved", Requirements: "approved", Design: "approved"}
	cases := []struct {
		name          string
		mode          string
		gates         Gates
		phase, status string
		want          string // substring of the violation; "" = allowed
	}{
		{"first phase", "greenfield", Gates{}, "intake", "draft", ""},
		{"skips ahead", "greenfield", Gates{Intake: "approved"}, "design", "approved", "requires requirements"},
		{"await is not approved", "greenfield", Gates{Intake: "await-approval"}, "requirements", "draft", "requires intake"},
		{"greenfield skips explore", "greenfield", Gates{Intake: "approved"}, "requirements", "draft", ""},
		{"existing needs explore", "existing", Gates{Intake: "approved"}, "requirements", "draft", "requires explore"},
		{"explore not in greenfield", "greenfield", Gates{Intake: "approved"}, "explore", "draft", "does not apply"},
		{"full loop", "existing", approved, "plan", "approved", ""},
		{"clearing always allowed", "greenfield", Gates{}, "plan", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := State{Mode: c.mode, Gates: c.gates}
			err := CheckGate(st, c.phase, c.status)
			if c.want == "" {
				if err != nil {
					t.Fatalf("want allowed, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
		})
	}
}

func TestCorruptStateIsNeverOverwritten(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedChange(root, "1-x", "greenfield", nil); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte(`{"change": "1-x", "changes": {`)
	if err := os.WriteFile(StatePath(root), corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SwitchTo(root, "1-x"); err == nil {
		t.Fatal("SwitchTo must fail on corrupt state")
	}
	if _, err := SeedChange(root, "2-y", "greenfield", nil); err == nil {
		t.Fatal("SeedChange must fail on corrupt state")
	}
	if _, err := KnownChanges(root); err == nil {
		t.Fatal("KnownChanges must fail on corrupt state")
	}
	if got, _ := os.ReadFile(StatePath(root)); string(got) != string(corrupt) {
		t.Fatalf("state.json was overwritten: %s", got)
	}
}

// A clone has STATE.json but no local pointer: nothing is active until a switch,
// which restores everything that was recorded.
func TestCloneRestoresRecordOnSwitch(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedChange(root, "1-x", "existing", []string{"00_intake/1.md"}); err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceGate(root, "intake", "approved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(ActivePath(root)); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Change != "" {
		t.Fatalf("change = %q, want none active", st.Change)
	}
	st, err = SwitchTo(root, "1-x")
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode != "existing" || st.Gates.Intake != "approved" || len(st.Sources) != 1 {
		t.Fatalf("record not restored: %+v", st)
	}
}

func TestSwitchToRecordedChangeLeavesStateFileUntouched(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedChange(root, "1-x", "greenfield", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := SeedChange(root, "2-y", "greenfield", nil); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(StatePath(root))
	Now = fixedNow(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	defer func() { Now = time.Now }()
	if _, err := SwitchTo(root, "1-x"); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(StatePath(root)); string(after) != string(before) {
		t.Fatalf("switch dirtied STATE.json:\n%s", after)
	}
	if name, _ := ReadActive(root); name != "1-x" {
		t.Fatalf("active = %q", name)
	}
}

func TestAdvanceGateRefusesActiveChangeWithoutFolder(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedChange(root, "1-x", "greenfield", nil); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(ChangeDir(root, "1-x")); err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceGate(root, "intake", "approved"); err == nil || !strings.Contains(err.Error(), "no folder") {
		t.Fatalf("want no folder, got %v", err)
	}
}

func TestOldStateLocationIsReportedNotIgnored(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Workspace(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Workspace(root), "state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(root); err == nil || !strings.Contains(err.Error(), "found old") {
		t.Fatalf("want found old, got %v", err)
	}
}

func TestSeedChangeFolderSurvivesGit(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedChange(root, "1-x", "greenfield", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ChangeDir(root, "1-x"), ".gitkeep")); err != nil {
		t.Fatalf("empty change folder would be lost in a clone: %v", err)
	}
}
