package icm

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/btdstudio/kiso/scaffold"
)

// omittedLocally mirrors kiso's rule for a local workspace (vcs: none): every
// git file is left out except .gitignore, which keeps .zumen/active and
// .zumen/paths.json out of an enclosing or later git repo.
func omittedLocally(rel string) bool {
	return scaffold.IsGitFile(rel) && path.Base(rel) != ".gitignore"
}

// gitFilesOnDisk lists the files under dir that match, skipping .git itself.
func gitFilesOnDisk(t *testing.T, dir string, match func(string) bool) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(dir, p)
		if !d.IsDir() && match(filepath.ToSlash(rel)) {
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// manifestGitFiles lists the manifest entries that match.
func manifestGitFiles(t *testing.T, dir string, match func(string) bool) []string {
	t.Helper()
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for rel := range m.Files {
		if match(rel) {
			found = append(found, rel)
		}
	}
	return found
}

// reportGitFiles lists the report entries that match.
func reportGitFiles(rep UpdateReport, match func(string) bool) []string {
	var found []string
	for _, list := range [][]string{rep.Updated, rep.Skipped, rep.Unchanged} {
		for _, rel := range list {
			if match(rel) {
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

// requireGitignoreManaged fails unless .gitignore is on disk and in the manifest.
func requireGitignoreManaged(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err != nil {
		t.Fatalf(".gitignore missing: %v", err)
	}
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Files[".gitignore"]; !ok {
		t.Fatal(".gitignore not in manifest")
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

// N-0002 AC2: a local init has no git repo and keeps only .gitignore of the git
// files, and records "vcs": "none".
func TestScaffoldLocalKeepsOnlyGitignore(t *testing.T) {
	dir := localWorkspace(t)
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git exists (err = %v)", err)
	}
	requireGitignoreManaged(t, dir)
	if got := gitFilesOnDisk(t, dir, omittedLocally); len(got) != 0 {
		t.Fatalf("omitted git files on disk: %v", got)
	}
	if got := manifestGitFiles(t, dir, omittedLocally); len(got) != 0 {
		t.Fatalf("omitted git files in manifest: %v", got)
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
	if len(manifestGitFiles(t, dir, scaffold.IsGitFile)) == 0 {
		t.Fatal("manifest lists no git files")
	}
	if _, ok := projectSettings(t, dir)["vcs"]; ok {
		t.Fatal("settings has a vcs key")
	}
}

// N-0002 AC3: update on a local workspace made before kiso v0.8.0 (no .gitignore,
// none in the manifest) creates .gitignore, reports it and records it, and still
// writes and reports no other git file.
func TestUpdateLocalAddsGitignoreToV04Workspace(t *testing.T) {
	dir := localWorkspace(t)
	if err := os.Remove(filepath.Join(dir, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	delete(m.Files, ".gitignore")
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	requireGitignoreManaged(t, dir)
	if !slices.Contains(rep.Updated, ".gitignore") {
		t.Fatalf("Updated = %v, want .gitignore", rep.Updated)
	}
	if got := gitFilesOnDisk(t, dir, omittedLocally); len(got) != 0 {
		t.Fatalf("omitted git files on disk: %v", got)
	}
	if got := reportGitFiles(rep, omittedLocally); len(got) != 0 {
		t.Fatalf("report names omitted git files: %v", got)
	}
}

// N-0002 AC4: switching git → none leaves every git file on disk, keeps
// .gitignore managed and stops managing the rest.
func TestUpdateGitToNoneKeepsGitignoreManaged(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := scaffoldWorkspace(t)
	before := map[string]string{}
	for _, rel := range gitFilesOnDisk(t, dir, scaffold.IsGitFile) {
		data, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		before[rel] = string(data)
	}
	if len(before) == 0 || !slices.Contains(gitFilesOnDisk(t, dir, omittedLocally), ".gitattributes") {
		t.Fatalf("setup: git files scaffolded = %v", before)
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
	requireGitignoreManaged(t, dir)
	if got := manifestGitFiles(t, dir, omittedLocally); len(got) != 0 {
		t.Fatalf("omitted git files still in manifest: %v", got)
	}
	if got := reportGitFiles(rep, omittedLocally); len(got) != 0 {
		t.Fatalf("report names omitted git files: %v", got)
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
	// .gitignore is managed under none too, so check the files none leaves out.
	if got := manifestGitFiles(t, dir, omittedLocally); !slices.Contains(got, ".gitattributes") {
		t.Fatalf("manifest omitted-locally files = %v, want .gitattributes restored", got)
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
