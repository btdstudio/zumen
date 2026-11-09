package icm

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/kiso/scaffold"
)

// gitFilesOnDisk lists the .gitignore, .gitattributes and .gitkeep files under
// dir, skipping .git itself.
func gitFilesOnDisk(t *testing.T, dir string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(dir, path)
		if !d.IsDir() && scaffold.IsGitFile(filepath.ToSlash(rel)) {
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func manifestGitFiles(t *testing.T, dir string) []string {
	t.Helper()
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for rel := range m.Files {
		if scaffold.IsGitFile(rel) {
			found = append(found, rel)
		}
	}
	return found
}

func reportGitFiles(rep UpdateReport) []string {
	var found []string
	for _, list := range [][]string{rep.Updated, rep.Skipped, rep.Unchanged} {
		for _, rel := range list {
			if scaffold.IsGitFile(rel) {
				found = append(found, rel)
			}
		}
	}
	return found
}

func projectSettings(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".zumen", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// setVCS rewrites the project settings' "vcs" key, keeping the rest.
func setVCS(t *testing.T, dir, vcs string) {
	t.Helper()
	m := projectSettings(t, dir)
	m["vcs"] = vcs
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, ".zumen", "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func localWorkspace(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	if _, err := Scaffold(dir, Options{NoGit: true}, "none"); err != nil {
		t.Fatal(err)
	}
	return dir
}

// AC5: a local init has no git repo, no git files and records "vcs": "none".
func TestScaffoldLocalHasNoGit(t *testing.T) {
	dir := localWorkspace(t)
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git exists (err = %v)", err)
	}
	if got := gitFilesOnDisk(t, dir); len(got) != 0 {
		t.Fatalf("git files on disk: %v", got)
	}
	if got := manifestGitFiles(t, dir); len(got) != 0 {
		t.Fatalf("git files in manifest: %v", got)
	}
	if got := projectSettings(t, dir)["vcs"]; got != "none" {
		t.Fatalf("settings vcs = %v, want none", got)
	}
}

// AC6: a default init is a git repo with its git files and no "vcs" key.
func TestScaffoldGitKeepsGitFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	rep, err := Scaffold(dir, Options{}, "git")
	if err != nil {
		t.Fatal(err)
	}
	if !rep.GitInit {
		t.Fatal("no git init")
	}
	for _, rel := range []string{".git", ".gitignore", ".gitattributes"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("%s missing: %v", rel, err)
		}
	}
	if len(manifestGitFiles(t, dir)) == 0 {
		t.Fatal("manifest lists no git files")
	}
	if _, ok := projectSettings(t, dir)["vcs"]; ok {
		t.Fatal("settings has a vcs key")
	}
}

// AC7: update in a local workspace creates and reports no git file.
func TestUpdateLocalWritesNoGitFiles(t *testing.T) {
	dir := localWorkspace(t)
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := gitFilesOnDisk(t, dir); len(got) != 0 {
		t.Fatalf("git files on disk: %v", got)
	}
	if got := reportGitFiles(rep); len(got) != 0 {
		t.Fatalf("report names git files: %v", got)
	}
}

// AC8: switching git → none leaves git files on disk but stops managing them.
func TestUpdateGitToNoneLeavesGitFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := scaffoldWorkspace(t)
	before := map[string]string{}
	for _, rel := range gitFilesOnDisk(t, dir) {
		data, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		before[rel] = string(data)
	}
	if len(before) == 0 {
		t.Fatal("setup: no git files scaffolded")
	}
	setVCS(t, dir, "none")
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	for rel, body := range before {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil || string(data) != body {
			t.Fatalf("%s changed or removed (err = %v)", rel, err)
		}
	}
	if got := manifestGitFiles(t, dir); len(got) != 0 {
		t.Fatalf("git files still in manifest: %v", got)
	}
	if got := reportGitFiles(rep); len(got) != 0 {
		t.Fatalf("report names git files: %v", got)
	}
}

// AC9: switching none → git restores the git files and manages them again.
func TestUpdateNoneToGitRestoresGitFiles(t *testing.T) {
	dir := localWorkspace(t)
	setVCS(t, dir, "git")
	if _, err := Update(dir); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".gitignore", ".gitattributes"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("%s not restored: %v", rel, err)
		}
	}
	if len(manifestGitFiles(t, dir)) == 0 {
		t.Fatal("manifest lists no git files")
	}
}

// An invalid "vcs" fails update before any v1 room is moved.
func TestUpdateInvalidVCSFailsBeforeMigrating(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	writeV1(t, dir, map[string]string{
		"00_intake/context.md": "old intake",
		"04_plan/context.md":   "old plan",
	})
	if err := os.WriteFile(filepath.Join(dir, ".zumen", "settings.json"), []byte(`{"vcs": "svn"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Update(dir)
	if err == nil || !strings.Contains(err.Error(), "vcs") || !strings.Contains(err.Error(), "svn") {
		t.Fatalf("err = %v, want an invalid vcs error", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "04_plan", "context.md")); err != nil {
		t.Fatalf("v1 room moved despite the error: %v", err)
	}
}
