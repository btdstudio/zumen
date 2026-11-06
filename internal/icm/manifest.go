// Package icm implements zumen's ICM engine: scaffold a folder-driven planning
// workspace, refresh its managed files, and validate the scaffold.
package icm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/btdstudio/zumen"
	"github.com/btdstudio/zumen/internal/version"
)

// WorkspaceDir is the per-project config directory. It is committed, except for
// the local .zumen/active pointer.
const WorkspaceDir = ".zumen"

// Manifest records the zumen version and a sha256 per managed file, so
// `zumen update` can tell which files the user has edited.
type Manifest struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"` // relative path -> sha256 hex
}

// walkScaffold calls fn for each file in the embedded scaffold, passing the
// path relative to the scaffold root and the file's bytes.
func walkScaffold(fn func(rel string, data []byte) error) error {
	return fs.WalkDir(zumen.ScaffoldFS, "scaffold", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Skip OS junk (macOS .DS_Store) so the embedded scaffold and its
		// manifest are deterministic regardless of the build machine.
		if d.Name() == ".DS_Store" {
			return nil
		}
		data, err := fs.ReadFile(zumen.ScaffoldFS, p)
		if err != nil {
			return err
		}
		return fn(strings.TrimPrefix(p, "scaffold/"), data)
	})
}

func readScaffoldFile(rel string) ([]byte, error) {
	return fs.ReadFile(zumen.ScaffoldFS, "scaffold/"+rel)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// BuildManifest walks the embedded scaffold and hashes every file.
func BuildManifest() (Manifest, error) {
	m := Manifest{Version: version.String(), Files: map[string]string{}}
	err := walkScaffold(func(rel string, data []byte) error {
		m.Files[rel] = hashBytes(data)
		return nil
	})
	if err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func manifestPath(dir string) string {
	return filepath.Join(dir, WorkspaceDir, "manifest.json")
}

// ReadManifest reads .zumen/manifest.json.
func ReadManifest(dir string) (Manifest, error) {
	data, err := os.ReadFile(manifestPath(dir))
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	if m.Files == nil {
		m.Files = map[string]string{}
	}
	return m, nil
}

// WriteManifest writes .zumen/manifest.json.
func WriteManifest(dir string, m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(manifestPath(dir)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(manifestPath(dir), data, 0o644)
}
