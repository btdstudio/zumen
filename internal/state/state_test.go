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
	if err := SetGate(&st, "spec", "await-approval", ""); err != nil {
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
	if loaded.Phase != "spec" || loaded.Status != "await-approval" {
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
	if err := SetGate(&st, "architecture", "draft", ""); err != nil {
		t.Fatal(err)
	}
	if st.Phase != "architecture" || st.Status != "draft" || st.Gates["architecture"] != "draft" {
		t.Fatalf("phase/status/gates.design = %q/%q/%q", st.Phase, st.Status, st.Gates["architecture"])
	}
	if st.Gates["intake"] != "" || st.Gates["explore"] != "" || st.Gates["spec"] != "" || st.Gates["breakdown"] != "" {
		t.Fatalf("unrelated gates should stay empty, got %+v", st.Gates)
	}
}

func TestSetGateRejectsUnknownPhaseAndStatus(t *testing.T) {
	st := DefaultState()
	if err := SetGate(&st, "bogus", "draft", ""); err == nil || !strings.Contains(err.Error(), "unknown phase") {
		t.Fatalf("want unknown phase error, got %v", err)
	}
	if err := SetGate(&st, "breakdown", "nope", ""); err == nil || !strings.Contains(err.Error(), "unknown gate status") {
		t.Fatalf("want unknown gate status error, got %v", err)
	}
}

func TestSaveStateWritesSlimShapeAndLoadResolves(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-alpha"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root), "2-beta"), 0o755)

	a := DefaultState()
	a.Name = "1-alpha"
	if err := SetGate(&a, "spec", "approved", ""); err != nil {
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
	if err := SetGate(&b, "architecture", "draft", ""); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, &b); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(StatePath(root))
	if err != nil {
		t.Fatal(err)
	}
	var sf struct {
		Changes map[string]*State `json:"changes"`
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		t.Fatal(err)
	}
	// The active change is not shared.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	if _, ok := top["changes"]; !ok || len(top) != 2 || string(top["schema"]) != "2" {
		t.Fatalf("top-level keys wrong: %s", raw)
	}
	if name, err := ReadActive(root); err != nil || name != "2-beta" {
		t.Fatalf("active = %q, %v", name, err)
	}
	if sf.Changes["2-beta"].UpdatedAt == "" {
		t.Fatal("per-change updatedAt must be stamped")
	}
	if sf.Changes["1-alpha"].Gates["spec"] != "approved" {
		t.Fatalf("1-alpha requirements = %q", sf.Changes["1-alpha"].Gates["spec"])
	}
	if sf.Changes["2-beta"].Gates["architecture"] != "draft" {
		t.Fatalf("2-beta design = %q", sf.Changes["2-beta"].Gates["architecture"])
	}

	active, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if active.Name != "2-beta" || active.Phase != "architecture" || active.Status != "draft" {
		t.Fatalf("active = %+v", active)
	}

	switched, err := SwitchTo(root, "1-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if switched.Phase != "spec" || switched.Status != "approved" {
		t.Fatalf("switched phase/status = %q/%q", switched.Phase, switched.Status)
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
	st.Name = "1-auth-flow"
	if err := SetGate(&st, "breakdown", "approved", ""); err != nil {
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
	if other.Name != "2-refactor" || other.Phase != "" || other.Status != "" {
		t.Fatalf("fresh folder = %+v", other)
	}

	back, err := SwitchTo(root, "1-auth-flow")
	if err != nil {
		t.Fatal(err)
	}
	if back.Name != "1-auth-flow" || back.Phase != "breakdown" || back.Status != "approved" {
		t.Fatalf("back = %+v", back)
	}
	if !reflect.DeepEqual(back.Sources, []string{"00_intake/1.md"}) {
		t.Fatalf("back sources = %v", back.Sources)
	}
	if back.Gates["breakdown"] != "approved" {
		t.Fatalf("back gates.plan = %q", back.Gates["breakdown"])
	}
}

func TestChangeGates(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-auth-flow"), 0o755)
	st := DefaultState()
	st.Name = "1-auth-flow"
	if err := SetGate(&st, "breakdown", "approved", ""); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, &st); err != nil {
		t.Fatal(err)
	}
	gates, err := ChangeGates(root, "1-auth-flow")
	if err != nil {
		t.Fatal(err)
	}
	if gates["breakdown"] != "approved" {
		t.Fatalf("gates = %v", gates)
	}
	gates["breakdown"] = "draft"
	again, err := ChangeGates(root, "1-auth-flow")
	if err != nil {
		t.Fatal(err)
	}
	if again["breakdown"] != "approved" {
		t.Fatalf("mutating the result changed state: %v", again)
	}
	for _, name := range []string{"2-unrecorded", ""} {
		none, err := ChangeGates(root, name)
		if err != nil || none == nil || len(none) != 0 {
			t.Fatalf("ChangeGates(%q) = %v, %v; want an empty map", name, none, err)
		}
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
	st.Name = "1-a"
	if err := SetGate(&st, "spec", "await-approval", ""); err != nil {
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
	if err := AppendLog(root, "spec → await-approval"); err != nil {
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
	st.Name = "1-x"
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
	st, err := SeedChange(root, "1-draft-posts", true, []string{"00_intake/1.md"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "1-draft-posts" || st.Phase != "intake" || st.Status != "draft" {
		t.Fatalf("st = %+v", st)
	}
	if st.Gates["intake"] != "draft" {
		t.Fatalf("gates.intake = %q", st.Gates["intake"])
	}
	if _, err := os.Stat(filepath.Join(SpecsDir(root), "1-draft-posts")); err != nil {
		t.Fatalf("change folder not created: %v", err)
	}
	active, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if active.Name != "1-draft-posts" {
		t.Fatalf("active = %+v", active)
	}
}

func TestSeedChangeRejectsBadName(t *testing.T) {
	if _, err := SeedChange(t.TempDir(), "../evil", false, nil); err == nil || !strings.Contains(err.Error(), "invalid change name") {
		t.Fatalf("want invalid change name, got %v", err)
	}
}

func TestAdvanceGateTransitionsAndLogs(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedChange(root, "1-x", false, nil); err != nil {
		t.Fatal(err)
	}
	st, err := AdvanceGate(root, "spec", "await-approval", "")
	if err != nil {
		t.Fatal(err)
	}
	if st.Phase != "spec" || st.Status != "await-approval" || st.Gates["spec"] != "await-approval" {
		t.Fatalf("st = %+v", st)
	}
	b, _ := os.ReadFile(LogPath(root))
	if !strings.Contains(string(b), "spec → await-approval") {
		t.Fatalf("log missing transition: %q", b)
	}
}

func TestAdvanceGateRequiresActiveChange(t *testing.T) {
	if _, err := AdvanceGate(t.TempDir(), "spec", "approved", ""); err == nil || !strings.Contains(err.Error(), "no active change") {
		t.Fatalf("want no active change, got %v", err)
	}
}

func TestCorruptStateIsNeverOverwritten(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedChange(root, "1-x", false, nil); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte(`{"change": "1-x", "changes": {`)
	if err := os.WriteFile(StatePath(root), corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SwitchTo(root, "1-x"); err == nil {
		t.Fatal("SwitchTo must fail on corrupt state")
	}
	if _, err := SeedChange(root, "2-y", false, nil); err == nil {
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
	if _, err := SeedChange(root, "1-x", true, []string{"00_intake/1.md"}); err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceGate(root, "intake", "approved", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(ActivePath(root)); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "" {
		t.Fatalf("change = %q, want none active", st.Name)
	}
	st, err = SwitchTo(root, "1-x")
	if err != nil {
		t.Fatal(err)
	}
	if st.Gates["explore"] != "" || st.Gates["intake"] != "approved" || len(st.Sources) != 1 {
		t.Fatalf("record not restored: %+v", st)
	}
}

func TestSwitchToRecordedChangeLeavesStateFileUntouched(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedChange(root, "1-x", false, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := SeedChange(root, "2-y", false, nil); err != nil {
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
	if _, err := SeedChange(root, "1-x", false, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(ChangeDir(root, "1-x")); err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceGate(root, "intake", "approved", ""); err == nil || !strings.Contains(err.Error(), "no folder") {
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
	if _, err := SeedChange(root, "1-x", false, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ChangeDir(root, "1-x"), ".gitkeep")); err != nil {
		t.Fatalf("empty change folder would be lost in a clone: %v", err)
	}
}

func TestPhasesAndDefaults(t *testing.T) {
	var names []string
	for _, p := range Machine.Phases {
		names = append(names, p.Name)
	}
	if !reflect.DeepEqual(names, []string{"intake", "research", "explore", "synthesis", "spec", "architecture", "breakdown", "publish"}) {
		t.Fatalf("phases = %v", names)
	}
	if len(DefaultState().Gates) != 0 {
		t.Fatalf("gates should be empty, got %+v", DefaultState().Gates)
	}
}

func TestCheckGate(t *testing.T) {
	approved := map[string]string{"intake": "approved", "research": "skipped", "explore": "approved", "synthesis": "skipped", "spec": "approved", "architecture": "approved"}
	cases := []struct {
		name          string
		gates         map[string]string
		phase, status string
		want          string // substring of the violation; "" = allowed
	}{
		{"first phase", nil, "intake", "draft", ""},
		{"skips ahead", map[string]string{"intake": "approved", "research": "skipped", "explore": "skipped", "synthesis": "skipped"}, "architecture", "approved", "requires spec"},
		{"await is not approved", map[string]string{"intake": "await-approval"}, "research", "draft", "requires intake"},
		{"skipped phases pass", map[string]string{"intake": "approved", "research": "skipped", "explore": "skipped", "synthesis": "skipped"}, "spec", "draft", ""},
		{"unskipped explore blocks", map[string]string{"intake": "approved", "research": "skipped"}, "spec", "draft", "requires explore"},
		{"full loop", approved, "breakdown", "approved", ""},
		{"clearing always allowed", nil, "breakdown", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckGate(State{Gates: c.gates}, c.phase, c.status)
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

func TestSeedChangeSkipsExploreWithoutRepo(t *testing.T) {
	root := t.TempDir()
	st, err := SeedChange(root, "1-x", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.Gates["explore"] != "skipped" {
		t.Fatalf("no repo: gates = %+v", st.Gates)
	}
	st, err = SeedChange(root, "2-y", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.Gates["explore"] != "" {
		t.Fatalf("repo: gates = %+v", st.Gates)
	}
}

// A STATE.json from before schema 1 keeps every approval, and `mode` becomes
// whether explore is skipped.

func TestSeedChangeSkipsSynthesisUnlessSeveralRepos(t *testing.T) {
	root := t.TempDir()
	one, err := SeedChange(root, "1-x", true, nil, "a")
	if err != nil {
		t.Fatal(err)
	}
	if one.Gates["synthesis"] != "skipped" {
		t.Fatalf("one repo: %+v", one.Gates)
	}
	two, err := SeedChange(root, "2-y", true, nil, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if two.Gates["synthesis"] != "" || two.Gates["research"] != "" {
		t.Fatalf("two repos: %+v", two.Gates)
	}
}

func readMigrated(t *testing.T, body string) map[string]*State {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(StatePath(root), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	changes, err := ReadChanges(root)
	if err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(StatePath(root)); string(after) != body {
		t.Fatal("a read rewrote STATE.json")
	}
	return changes
}

// A STATE.json from before schema 1 keeps every approval: `mode` becomes whether
// explore is skipped, and v1's phases are renamed.
func TestMigrateFromMode(t *testing.T) {
	changes := readMigrated(t, `{
  "changes": {
    "1-idea": {"phase":"plan","status":"approved","mode":"greenfield","sources":["00_intake/1.md"],"updatedAt":"2026-10-01T00:00:00.000Z",
      "gates":{"intake":"approved","explore":"","requirements":"approved","design":"approved","plan":"approved"}},
    "2-repo": {"phase":"requirements","status":"draft","mode":"existing","sources":null,"updatedAt":"2026-10-02T00:00:00.000Z",
      "gates":{"intake":"approved","explore":"approved","requirements":"draft","design":"","plan":""}},
    "3-repo": {"phase":"intake","status":"approved","mode":"existing","sources":[],"updatedAt":"2026-10-03T00:00:00.000Z",
      "gates":{"intake":"approved","explore":"","requirements":"","design":"","plan":""}}
  }
}`)
	want := map[string]map[string]string{
		"1-idea": {"intake": "approved", "research": "skipped", "explore": "skipped", "synthesis": "skipped", "spec": "approved", "architecture": "approved", "breakdown": "approved"},
		"2-repo": {"intake": "approved", "research": "skipped", "explore": "approved", "synthesis": "skipped", "spec": "draft"},
		"3-repo": {"intake": "approved", "synthesis": "skipped"},
	}
	for name, gates := range want {
		if !reflect.DeepEqual(changes[name].Gates, gates) {
			t.Errorf("%s gates = %+v, want %+v", name, changes[name].Gates, gates)
		}
	}
	if changes["1-idea"].Phase != "breakdown" || changes["2-repo"].Phase != "spec" {
		t.Errorf("phases not renamed: %q, %q", changes["1-idea"].Phase, changes["2-repo"].Phase)
	}
	if got := CheckGate(*changes["1-idea"], "publish", "in-progress"); got != nil {
		t.Errorf("an approved plan should reach publish: %v", got)
	}
	if got := CheckGate(*changes["3-repo"], "spec", "draft"); got == nil {
		t.Error("a change still has to do research and explore")
	}
}

func TestMigrateFromSchema1(t *testing.T) {
	changes := readMigrated(t, `{"schema":1,"changes":{
  "1-past":{"phase":"plan","status":"draft","gates":{"intake":"approved","explore":"skipped","requirements":"approved","design":"approved","plan":"draft"},"sources":[]},
  "2-new":{"phase":"intake","status":"draft","gates":{"intake":"draft","explore":"skipped"},"sources":[]},
  "3-multi":{"phase":"intake","status":"approved","gates":{"intake":"approved"},"sources":[],"repos":["a","b"]},
  "4-closed":{"phase":"requirements","status":"closed","gates":{"intake":"approved","explore":"skipped","requirements":"draft"},"sources":[]}
}}`)
	want := map[string]map[string]string{
		"1-past":   {"intake": "approved", "research": "skipped", "explore": "skipped", "synthesis": "skipped", "spec": "approved", "architecture": "approved", "breakdown": "draft"},
		"2-new":    {"intake": "draft", "explore": "skipped", "synthesis": "skipped"},
		"3-multi":  {"intake": "approved"},
		"4-closed": {"intake": "approved", "research": "skipped", "explore": "skipped", "synthesis": "skipped", "spec": "draft"},
	}
	for name, gates := range want {
		if !reflect.DeepEqual(changes[name].Gates, gates) {
			t.Errorf("%s gates = %+v, want %+v", name, changes[name].Gates, gates)
		}
	}
	if !changes["4-closed"].IsClosed() || changes["4-closed"].Phase != "spec" {
		t.Errorf("closed change = %+v", changes["4-closed"])
	}
}

func TestMigrationPersistsOnNextWrite(t *testing.T) {
	root := t.TempDir()
	body := `{"schema":1,"changes":{"1-x":{"phase":"design","status":"draft","gates":{"intake":"approved","explore":"skipped","requirements":"approved","design":"draft"}}}}`
	if err := os.WriteFile(StatePath(root), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ChangeDir(root, "2-y"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := SwitchTo(root, "2-y"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(StatePath(root))
	for _, want := range []string{`"schema": 2`, `"architecture": "draft"`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("missing %s in:\n%s", want, raw)
		}
	}
	if strings.Contains(string(raw), `"design"`) {
		t.Fatalf("old phase remains:\n%s", raw)
	}
}

func TestMachineCheckIsWired(t *testing.T) {
	if Machine.Check == nil || store.Machine.Check == nil {
		t.Fatal("Machine.Check isn't set, or the store's copy lost it")
	}
	root := t.TempDir()
	if ps := store.Machine.CheckArtifacts(root, "1-x", "architecture", "approved"); len(ps) != 1 || ps[0].Message != "doesn't exist" {
		t.Fatalf("store's CheckArtifacts = %v", ps)
	}
	if ps := Machine.CheckArtifacts(root, "1-x", "architecture", "await-approval"); ps != nil {
		t.Fatalf("await-approval = %v", ps)
	}
}
