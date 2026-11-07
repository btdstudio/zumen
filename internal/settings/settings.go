// Package settings defines zumen's settings shape and merges it through kiso:
// effective settings = defaults ← global (~/.config/zumen/settings.json) ←
// project (.zumen/settings.json).
package settings

import (
	core "github.com/btdstudio/kiso/settings"
)

// Settings is the effective settings shape.
type Settings struct {
	BaseSource  string `json:"baseSource"`
	Gates       string `json:"gates"`
	SourceDir   string `json:"sourceDir"`
	ArtifactDir string `json:"artifactDir"`
	AdrDir      string `json:"adrDir"`
}

// Loaded is the result of LoadSettings.
type Loaded = core.Loaded[Settings]

var spec = core.Spec[Settings]{
	Tool:     "zumen",
	Defaults: DefaultSettings,
	Apply:    apply,
	Validate: func(s *Settings) error { return core.CheckGates("zumen", s.Gates) },
}

func DefaultSettings() Settings {
	return Settings{
		BaseSource:  "",                    // default base source: a 00_intake path | "issue:N" | "url:..."
		Gates:       "block",               // block | prompt | auto
		SourceDir:   "00_intake",           // the intake store
		ArtifactDir: "specs",               // per-change artifacts
		AdrDir:      "04_architecture/adr", // architecture decision records
	}
}

func apply(s *Settings, key string, value any) {
	val, ok := value.(string)
	if !ok {
		return
	}
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

func GlobalSettingsPath() string { return spec.GlobalPath() }

func ProjectSettingsPath(root string) string { return spec.ProjectPath(root) }

// ReadIfExists parses a JSON object file, returning an empty map when absent.
func ReadIfExists(file string) (map[string]any, error) { return spec.ReadIfExists(file) }

// MergeSettings overlays the raw layers (global then project) over defaults.
func MergeSettings(layers ...map[string]any) (Settings, error) { return spec.Merge(layers...) }

func LoadSettings(root string) (Loaded, error) { return spec.Load(root) }
