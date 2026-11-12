// Package icm configures kiso's scaffold engine for zumen: the embedded planning
// workspace and its generated seeds.
package icm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/btdstudio/kiso/scaffold"
	"github.com/btdstudio/zumen"
	"github.com/btdstudio/zumen/internal/settings"
)

// WorkspaceDir is the per-project config directory. It is committed, except for
// the local .zumen/active pointer.
const WorkspaceDir = ".zumen"

type (
	Manifest = scaffold.Manifest
	Options  = scaffold.Options
	Report   = scaffold.Report
)

// UpdateReport is kiso's report, plus what zumen did to AGENTS.md. A merged file
// leaves Skipped, because kiso skipped it as edited and zumen then handled it.
type UpdateReport struct {
	scaffold.UpdateReport
	Merged    []string          // files whose zumen-owned sections were merged in
	NotMerged map[string]string // files zumen couldn't merge, and why; they stay in Skipped
}

var engine = scaffold.Engine{
	Tool:   "zumen",
	FS:     mustSub(zumen.ScaffoldFS, "scaffold"),
	Marker: "00_intake",
	Seeds:  seeds,
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// seeds generates settings, STATE.json and LOG.md. They are produced by the
// binary — not copied from the scaffold — so `zumen update` never touches the
// user's workspace state.
func seeds() ([]scaffold.Seed, error) {
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
	return []scaffold.Seed{
		{Rel: ".zumen/settings.json", Data: append(settingsData, '\n')},
		{Rel: "STATE.json", Data: append(stateData, '\n')},
		{Rel: "LOG.md", Data: []byte{}},
	}, nil
}

// Scaffold writes the embedded tree into dir; see scaffold.Engine.Scaffold. vcs
// is the new workspace's "vcs" setting ("" means git); under "none" there's no
// git init and no git files, and the project settings record "vcs": "none".
func Scaffold(dir string, opts Options, vcs string) (Report, error) {
	e := engine
	e.VCS = vcs
	return e.Scaffold(dir, opts)
}

// Update moves v1 rooms to their v2 names, then refreshes managed files under the
// workspace's "vcs" setting; see scaffold.Engine.Update. An invalid setting fails
// before anything moves. An AGENTS.md that kiso skipped as edited gets zumen's
// sections merged into it; see MergeAgents.
func Update(dir string) (UpdateReport, error) {
	loaded, err := settings.LoadSettings(dir)
	if err != nil {
		return UpdateReport{}, err
	}
	if _, err := MigrateRooms(dir); err != nil {
		return UpdateReport{}, err
	}
	e := engine
	e.VCS = loaded.VCS
	kiso, err := e.Update(dir)
	rep := UpdateReport{UpdateReport: kiso}
	if err != nil {
		return rep, err
	}
	return rep, mergeAgentsFile(dir, &rep)
}

const agentsFile = "AGENTS.md"

// mergeAgentsFile merges the scaffold's AGENTS.md into the user's when kiso
// skipped it. A file that can't be merged is left alone and stays in Skipped. A
// merge that changes nothing counts as unchanged.
func mergeAgentsFile(dir string, rep *UpdateReport) error {
	i := slices.Index(rep.Skipped, agentsFile)
	if i < 0 {
		return nil
	}
	path := filepath.Join(dir, agentsFile)
	user, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	scaffoldText, err := fs.ReadFile(engine.FS, agentsFile)
	if err != nil {
		return err
	}
	merged, reason := MergeAgents(string(user), string(scaffoldText))
	if reason != "" {
		if rep.NotMerged == nil {
			rep.NotMerged = map[string]string{}
		}
		rep.NotMerged[agentsFile] = reason
		return nil
	}
	rep.Skipped = slices.Delete(rep.Skipped, i, i+1)
	if merged == string(user) {
		rep.Unchanged = append(rep.Unchanged, agentsFile)
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(merged), info.Mode().Perm()); err != nil {
		return err
	}
	rep.Merged = append(rep.Merged, agentsFile)
	return nil
}

// renamedRooms maps v1 room folders to their v2 names, in an order where a new name
// is free by the time it's used.
var renamedRooms = [][2]string{
	{"04_plan", "05_breakdown"},
	{"03_design", "04_architecture"},
	{"02_requirements", "03_spec"},
	{"01_explore", "02_explore"},
}

// MigrateRooms moves v1 room folders to their v2 names and returns what it moved.
// The manifest follows, so `update` can still tell unedited files from edited ones.
// Files the v2 scaffold no longer has are removed if unedited and kept if edited. A
// room whose new name is already taken is left alone.
func MigrateRooms(dir string) ([]string, error) {
	var moved [][2]string
	for _, room := range renamedRooms {
		from, to := filepath.Join(dir, room[0]), filepath.Join(dir, room[1])
		if info, err := os.Stat(from); err != nil || !info.IsDir() {
			continue
		}
		if _, err := os.Stat(to); err == nil {
			continue
		}
		if err := os.Rename(from, to); err != nil {
			return nil, err
		}
		moved = append(moved, room)
	}
	if len(moved) == 0 {
		return nil, nil
	}
	if err := fixAdrDir(dir); err != nil {
		return nil, err
	}
	var names []string
	for _, room := range moved {
		names = append(names, room[0]+" -> "+room[1])
	}
	cur, err := engine.ReadManifest(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	next, err := engine.BuildManifest()
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for rel, hash := range cur.Files {
		newRel := rel
		for _, room := range moved {
			if strings.HasPrefix(rel, room[0]+"/") {
				newRel = room[1] + strings.TrimPrefix(rel, room[0])
				if err := dropUneditedOrphan(dir, newRel, hash, next); err != nil {
					return nil, err
				}
			}
		}
		files[newRel] = hash
	}
	cur.Files = files
	return names, engine.WriteManifest(dir, cur)
}

func dropUneditedOrphan(dir, rel, recorded string, next Manifest) error {
	if _, stillManaged := next.Files[rel]; stillManaged {
		return nil
	}
	path := filepath.Join(dir, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != recorded {
		return nil
	}
	return os.Remove(path)
}

// fixAdrDir moves the seeded v1 default in a committed settings file. A custom value
// is left alone.
func fixAdrDir(dir string) error {
	path := settings.ProjectSettingsPath(dir)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	updated := strings.Replace(string(data), `"adrDir": "03_design/adr"`, `"adrDir": "04_architecture/adr"`, 1)
	if updated == string(data) {
		return nil
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}

// Check validates the embedded scaffold; see scaffold.Engine.Check.
func Check() []string { return engine.Check() }

// BuildManifest hashes the embedded scaffold.
func BuildManifest() (Manifest, error) { return engine.BuildManifest() }

// ReadManifest reads .zumen/manifest.json.
func ReadManifest(dir string) (Manifest, error) { return engine.ReadManifest(dir) }

// WriteManifest writes .zumen/manifest.json.
func WriteManifest(dir string, m Manifest) error { return engine.WriteManifest(dir, m) }
