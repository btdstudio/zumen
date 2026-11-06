package icm

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/btdstudio/zumen/internal/settings"
)

// Options configure Scaffold.
type Options struct {
	NoGit bool
	Force bool
}

// Report describes what Scaffold did.
type Report struct {
	Written []string
	GitInit bool
}

// Scaffold writes the embedded ICM tree into dir, skipping existing files
// (unless Force), initializing git when greenfield, and seeding the workspace
// state + .zumen/manifest.json.
func Scaffold(dir string, opts Options) (Report, error) {
	rep := Report{}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return rep, err
	}
	if !opts.NoGit {
		if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
			if err := runGit(dir, "init"); err != nil {
				return rep, err
			}
			rep.GitInit = true
		}
	}
	err := walkScaffold(func(rel string, data []byte) error {
		dest := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Stat(dest); err == nil && !opts.Force {
			return nil // skip existing user-owned file
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
		rep.Written = append(rep.Written, rel)
		return nil
	})
	if err != nil {
		return rep, err
	}
	seeded, err := seedWorkspace(dir)
	if err != nil {
		return rep, err
	}
	rep.Written = append(rep.Written, seeded...)
	m, err := BuildManifest()
	if err != nil {
		return rep, err
	}
	if err := WriteManifest(dir, m); err != nil {
		return rep, err
	}
	return rep, nil
}

// seedWorkspace writes the generated seeds (settings, STATE.json, LOG.md).
// They are produced by the binary — not copied from the scaffold — so `zumen
// update` never touches the user's workspace state. Existing files are skipped.
func seedWorkspace(dir string) ([]string, error) {
	settingsData, err := json.MarshalIndent(settings.DefaultSettings(), "", "  ")
	if err != nil {
		return nil, err
	}
	stateData, err := json.MarshalIndent(map[string]any{
		"changes": map[string]any{},
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	type seed struct {
		rel  string
		data []byte
	}
	seeds := []seed{
		{".zumen/settings.json", append(settingsData, '\n')},
		{"STATE.json", append(stateData, '\n')},
		{"LOG.md", []byte{}},
	}
	var written []string
	for _, s := range seeds {
		dest := filepath.Join(dir, filepath.FromSlash(s.rel))
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dest, s.data, 0o644); err != nil {
			return nil, err
		}
		written = append(written, s.rel)
	}
	return written, nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return nil
}
