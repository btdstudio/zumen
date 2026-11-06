package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportCopiesSpecFolder(t *testing.T) {
	root := t.TempDir()
	specs := filepath.Join(root, "specs", "1-x")
	os.MkdirAll(specs, 0o755)
	os.WriteFile(filepath.Join(specs, "requirements.md"), []byte("req"), 0o644)
	os.WriteFile(filepath.Join(specs, "design.md"), []byte("des"), 0o644)
	os.WriteFile(filepath.Join(specs, "tasks.json"), []byte("{}"), 0o644)

	target := t.TempDir()
	rep, err := Export(root, "1-x", target, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Change != "1-x" || len(rep.Copied) != 3 {
		t.Fatalf("rep = %+v", rep)
	}
	dst := filepath.Join(target, "01_planning", "specs", "1-x")
	if _, err := os.Stat(filepath.Join(dst, "tasks.json")); err != nil {
		t.Fatalf("tasks.json not exported: %v", err)
	}
}

func TestExportRejectsMissingChange(t *testing.T) {
	if _, err := Export(t.TempDir(), "9-nope", t.TempDir(), "", false); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestExportRejectsTraversalName(t *testing.T) {
	if _, err := Export(t.TempDir(), "../x", t.TempDir(), "", false); err == nil || !strings.Contains(err.Error(), "invalid change name") {
		t.Fatalf("want invalid change name, got %v", err)
	}
}

func TestExportHonorsCustomSpecsDir(t *testing.T) {
	root := t.TempDir()
	specs := filepath.Join(root, "specs", "1-x")
	os.MkdirAll(specs, 0o755)
	os.WriteFile(filepath.Join(specs, "tasks.json"), []byte("{}"), 0o644)
	target := t.TempDir()
	rep, err := Export(root, "1-x", target, ".tomo/specs", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rep.Dest, filepath.Join(".tomo", "specs", "1-x")) {
		t.Fatalf("dest = %q", rep.Dest)
	}
}

func TestExportRefusesExistingDestUnlessForced(t *testing.T) {
	root := t.TempDir()
	specs := filepath.Join(root, "specs", "1-x")
	os.MkdirAll(specs, 0o755)
	os.WriteFile(filepath.Join(specs, "tasks.json"), []byte("new"), 0o644)
	target := t.TempDir()
	dst := filepath.Join(target, "01_planning", "specs", "1-x")
	os.MkdirAll(dst, 0o755)
	os.WriteFile(filepath.Join(dst, "tasks.json"), []byte("old"), 0o644)

	if _, err := Export(root, "1-x", target, "", false); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("want already exists, got %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "tasks.json")); string(got) != "old" {
		t.Fatalf("refused export changed the dest: %q", got)
	}
	if _, err := Export(root, "1-x", target, "", true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "tasks.json")); string(got) != "new" {
		t.Fatalf("forced export did not overwrite: %q", got)
	}
}

func TestExportSkipsGitkeep(t *testing.T) {
	root := t.TempDir()
	specs := filepath.Join(root, "specs", "1-x")
	os.MkdirAll(specs, 0o755)
	os.WriteFile(filepath.Join(specs, ".gitkeep"), nil, 0o644)
	if _, err := Export(root, "1-x", t.TempDir(), "", false); err == nil || !strings.Contains(err.Error(), "no files") {
		t.Fatalf("want no files, got %v", err)
	}
	os.WriteFile(filepath.Join(specs, "tasks.json"), []byte("{}"), 0o644)
	rep, err := Export(root, "1-x", t.TempDir(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Copied) != 1 || rep.Copied[0] != "tasks.json" {
		t.Fatalf("copied = %v", rep.Copied)
	}
}
