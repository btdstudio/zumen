// Package settings owns zumen's mergeable settings: global
// (~/.config/zumen/settings.json) then project (.zumen/settings.json); the
// project supersedes the global. Effective settings = defaults ← global ←
// project.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/btdstudio/zumen/internal/state"
)

var VALID_GATES = []string{"block", "prompt", "auto"}

// Settings is the effective settings shape.
type Settings struct {
	BaseSource  string `json:"baseSource"`
	Gates       string `json:"gates"`
	SourceDir   string `json:"sourceDir"`
	ArtifactDir string `json:"artifactDir"`
	AdrDir      string `json:"adrDir"`
}

func DefaultSettings() Settings {
	return Settings{
		BaseSource:  "",              // default base source: a 00_intake path | "issue:N" | "url:..."
		Gates:       "block",         // block | prompt | auto
		SourceDir:   "00_intake",     // the intake store
		ArtifactDir: "specs",         // per-change artifacts
		AdrDir:      "03_design/adr", // architecture decision records
	}
}

func GlobalSettingsPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			base = ""
		} else {
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "zumen", "settings.json")
}

func ProjectSettingsPath(root string) string {
	return filepath.Join(state.Workspace(root), "settings.json")
}

// ReadIfExists parses a JSON object file, returning an empty map when absent.
func ReadIfExists(file string) (map[string]any, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("zumen: cannot parse settings %s: %v", file, err)
	}
	m, ok := parsed.(map[string]any)
	if !ok || m == nil {
		return nil, fmt.Errorf("zumen: settings must be a JSON object: %s", file)
	}
	return m, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func stringField(s *Settings, key, val string) {
	switch key {
	case "baseSource":
		s.BaseSource = val
	case "gates":
		s.Gates = val
	case "sourceDir":
		s.SourceDir = val
	case "artifactDir":
		s.ArtifactDir = val
	case "adrDir":
		s.AdrDir = val
	}
}

// MergeSettings overlays the raw layers (global then project) over defaults.
// Layers are raw JSON objects so a present-but-empty value still overrides.
func MergeSettings(layers ...map[string]any) (Settings, error) {
	out := DefaultSettings()
	for _, layer := range layers {
		for key, value := range layer {
			if value == nil {
				continue
			}
			if s, ok := value.(string); ok {
				stringField(&out, key, s)
			}
		}
	}
	if !contains(VALID_GATES, out.Gates) {
		return Settings{}, fmt.Errorf("zumen: invalid gates %q (expected block|prompt|auto)", out.Gates)
	}
	return out, nil
}

// Loaded is the result of LoadSettings.
type Loaded struct {
	Global    map[string]any
	Project   map[string]any
	Effective Settings
}

func LoadSettings(root string) (Loaded, error) {
	global, err := ReadIfExists(GlobalSettingsPath())
	if err != nil {
		return Loaded{}, err
	}
	project, err := ReadIfExists(ProjectSettingsPath(root))
	if err != nil {
		return Loaded{}, err
	}
	effective, err := MergeSettings(global, project)
	if err != nil {
		return Loaded{}, err
	}
	return Loaded{Global: global, Project: project, Effective: effective}, nil
}
