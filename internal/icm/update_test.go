package icm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scaffoldWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := Scaffold(dir, Options{NoGit: true}, ""); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestUpdateSkipsEditedAndRestoresMissing(t *testing.T) {
	dir := scaffoldWorkspace(t)
	agents := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Skipped) != 1 || rep.Skipped[0] != "AGENTS.md" {
		t.Fatalf("skipped = %v", rep.Skipped)
	}
	if len(rep.Updated) != 1 || rep.Updated[0] != "README.md" {
		t.Fatalf("updated = %v", rep.Updated)
	}
	if got, _ := os.ReadFile(agents); string(got) != "mine" {
		t.Fatalf("edit clobbered: %q", got)
	}
}

func TestUpdateReplacesUneditedFileFromOlderVersion(t *testing.T) {
	dir := scaffoldWorkspace(t)
	readme := filepath.Join(dir, "README.md")
	old := []byte("older scaffold")
	if err := os.WriteFile(readme, old, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.Files["README.md"] = hashBytes(old)
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Updated) != 1 || rep.Updated[0] != "README.md" {
		t.Fatalf("updated = %v", rep.Updated)
	}
	if got, _ := os.ReadFile(readme); string(got) == string(old) {
		t.Fatal("README.md was not refreshed")
	}
}

// A deleted .zumen/ must not cost the user their edits.
func TestUpdateRebuildsMissingManifestWithoutClobbering(t *testing.T) {
	dir := scaffoldWorkspace(t)
	if err := os.RemoveAll(filepath.Join(dir, WorkspaceDir)); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Skipped) != 1 || rep.Skipped[0] != "AGENTS.md" || len(rep.Updated) != 0 {
		t.Fatalf("rep = %+v", rep)
	}
	if got, _ := os.ReadFile(agents); string(got) != "mine" {
		t.Fatalf("edit clobbered: %q", got)
	}
	if _, err := ReadManifest(dir); err != nil {
		t.Fatalf("manifest not rebuilt: %v", err)
	}
}

func TestUpdateRefusesNonWorkspace(t *testing.T) {
	dir := t.TempDir()
	if _, err := Update(dir); err == nil || !strings.Contains(err.Error(), "not a zumen workspace") {
		t.Fatalf("want not a zumen workspace, got %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("wrote into a non-workspace: %v", entries)
	}
}

func TestScaffoldSeedsStateAtRootAndIgnoresOnlyActive(t *testing.T) {
	dir := scaffoldWorkspace(t)
	for _, rel := range []string{"STATE.json", "LOG.md", ".gitattributes", ".zumen/settings.json", ".zumen/manifest.json"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s not seeded: %v", rel, err)
		}
	}
	ignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(ignore), "\n")
	var active, whole bool
	for _, l := range lines {
		active = active || l == ".zumen/active"
		whole = whole || l == ".zumen/"
	}
	if !active || whole {
		t.Fatalf(".gitignore must ignore only .zumen/active:\n%s", ignore)
	}
}

// writeV1 lays out a v1 workspace by hand, its manifest recording each file as written.
func writeV1(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	m := Manifest{Version: "v0.2.0", Files: map[string]string{}}
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		m.Files[rel] = hashBytes([]byte(body))
	}
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMovesV1Rooms(t *testing.T) {
	dir := t.TempDir()
	writeV1(t, dir, map[string]string{
		"00_intake/context.md":                      "old intake",
		"01_explore/context.md":                     "old explore",
		"01_explore/findings/.gitkeep":              "",
		"02_requirements/context.md":                "old requirements",
		"02_requirements/templates/requirements.md": "old template",
		"03_design/context.md":                      "old design",
		"03_design/adr/.gitkeep":                    "",
		"03_design/templates/design.md":             "old design template",
		"04_plan/context.md":                        "old plan",
		"04_plan/templates/tasks.md":                "old tasks template",
		"AGENTS.md":                                 "old agents",
	})
	// Things a user made or changed in v1.
	for rel, body := range map[string]string{
		"03_design/adr/0001-use-go.md":        "a decision",
		"01_explore/findings/1-thing.md":      "findings",
		"02_requirements/personas/analyst.md": "my analyst",
	} {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	settingsPath := filepath.Join(dir, ".zumen", "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"adrDir": "03_design/adr", "gates": "block"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, old := range []string{"01_explore", "02_requirements", "03_design", "04_plan"} {
		if _, err := os.Stat(filepath.Join(dir, old)); !os.IsNotExist(err) {
			t.Errorf("%s still exists: %v", old, err)
		}
	}
	for rel, want := range map[string]string{
		"04_architecture/adr/0001-use-go.md": "a decision",
		"02_explore/findings/1-thing.md":     "findings",
	} {
		if got, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel))); string(got) != want {
			t.Errorf("%s = %q", rel, got)
		}
	}
	// Unedited managed files are refreshed, because the manifest followed the move.
	current, err := os.ReadFile(filepath.Join(dir, "03_spec", "context.md"))
	if err != nil || !strings.Contains(string(current), "03_spec") {
		t.Errorf("03_spec/context.md not refreshed: %q, %v", current, err)
	}
	// A file v1 didn't record, now at a managed path, is treated as edited.
	if got, _ := os.ReadFile(filepath.Join(dir, "03_spec", "personas", "analyst.md")); string(got) != "my analyst" {
		t.Errorf("edited persona clobbered: %q", got)
	}
	found := false
	for _, rel := range rep.Skipped {
		found = found || rel == "03_spec/personas/analyst.md"
	}
	if !found {
		t.Errorf("edited persona not reported: %v", rep.Skipped)
	}
	// Unedited files v2 dropped are gone.
	for _, rel := range []string{"05_breakdown/templates/tasks.md", "04_architecture/templates/design.md", "03_spec/templates/requirements.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s should be gone: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "05_breakdown", "templates", "ticket.md")); err != nil {
		t.Errorf("new template missing: %v", err)
	}
	if got, _ := os.ReadFile(settingsPath); !strings.Contains(string(got), `"adrDir": "04_architecture/adr"`) {
		t.Errorf("settings = %s", got)
	}
	// Running it again changes nothing about the layout.
	if moved, err := MigrateRooms(dir); err != nil || len(moved) != 0 {
		t.Errorf("second migration = %v, %v", moved, err)
	}
}

func TestMigrateRoomsKeepsEditedOrphansAndCustomSettings(t *testing.T) {
	dir := t.TempDir()
	writeV1(t, dir, map[string]string{"00_intake/context.md": "x", "04_plan/templates/tasks.md": "old"})
	if err := os.WriteFile(filepath.Join(dir, "04_plan", "templates", "tasks.md"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(dir, ".zumen", "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"adrDir": "decisions"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateRooms(dir); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "05_breakdown", "templates", "tasks.md")); string(got) != "mine" {
		t.Errorf("edited file lost: %q", got)
	}
	if got, _ := os.ReadFile(settingsPath); !strings.Contains(string(got), "decisions") {
		t.Errorf("custom adrDir changed: %s", got)
	}
}

func TestMigrateRoomsLeavesARoomWhoseNameIsTaken(t *testing.T) {
	dir := t.TempDir()
	for _, room := range []string{"04_plan", "05_breakdown"} {
		if err := os.MkdirAll(filepath.Join(dir, room), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	moved, err := MigrateRooms(dir)
	if err != nil || len(moved) != 0 {
		t.Fatalf("moved = %v, %v", moved, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "04_plan")); err != nil {
		t.Fatal(err)
	}
}

func TestSyncFileEditsSurviveUpdate(t *testing.T) {
	dir := scaffoldWorkspace(t)
	sync := filepath.Join(dir, "06_publish", "sync.md")
	if err := os.WriteFile(sync, []byte("my instructions"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(dir); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(sync); string(got) != "my instructions" {
		t.Fatalf("sync.md clobbered: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "06_publish", "sync-examples", "github.md")); err != nil {
		t.Fatal(err)
	}
}
