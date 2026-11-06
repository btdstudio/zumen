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
	if _, err := Scaffold(dir, Options{NoGit: true}); err != nil {
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
