package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeSettings(t *testing.T, p string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultsApplyWhenNoFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	loaded, err := LoadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Global, map[string]any{}) {
		t.Fatalf("global = %v", loaded.Global)
	}
	if !reflect.DeepEqual(loaded.Project, map[string]any{}) {
		t.Fatalf("project = %v", loaded.Project)
	}
	if !reflect.DeepEqual(loaded.Effective, DefaultSettings()) {
		t.Fatalf("effective = %+v", loaded.Effective)
	}
}

func TestProjectSupersedesGlobal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	globalPath := GlobalSettingsPath()
	projectPath := ProjectSettingsPath(root)
	writeSettings(t, globalPath, map[string]any{
		"baseSource": "00_intake/2.md", "gates": "auto",
		"sourceDir": "intake", "artifactDir": "plans",
	})
	writeSettings(t, projectPath, map[string]any{"gates": "prompt", "sourceDir": "custom"})
	loaded, err := LoadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	e := loaded.Effective
	if e.BaseSource != "00_intake/2.md" || e.Gates != "prompt" ||
		e.SourceDir != "custom" || e.ArtifactDir != "plans" || e.AdrDir != "03_design/adr" {
		t.Fatalf("effective = %+v", e)
	}
}

func TestInvalidGatesThrows(t *testing.T) {
	if _, err := MergeSettings(map[string]any{"gates": "aggressive"}); err == nil || !strings.Contains(err.Error(), "invalid gates") {
		t.Fatalf("want invalid gates, got %v", err)
	}
}

func TestUnreadableJSONThrows(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	p := ProjectSettingsPath(root)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(root); err == nil || !strings.Contains(err.Error(), "cannot parse settings") {
		t.Fatalf("want cannot parse settings, got %v", err)
	}
}

func TestGlobalPathHonorsXDGConfigHome(t *testing.T) {
	alt := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", alt)
	if !strings.HasPrefix(GlobalSettingsPath(), alt) {
		t.Fatalf("global path %q should start with %q", GlobalSettingsPath(), alt)
	}
	if !strings.HasSuffix(GlobalSettingsPath(), filepath.Join("zumen", "settings.json")) {
		t.Fatalf("global path %q should end with zumen/settings.json", GlobalSettingsPath())
	}
}
