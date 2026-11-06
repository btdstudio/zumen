// Package export copies a change's spec folder into a tomo workspace's
// planning specs dir, realizing the zumen → tomo handoff contract.
//
// zumen writes specs at specs/<n>-<slug>/{requirements,design,tasks}; tomo reads
// the same triple at 01_planning/specs/<n>-<slug>/. Export bridges the two by
// copying the folder, so the artifacts stay the single source of truth.
package export

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/btdstudio/zumen/internal/state"
)

// DefaultSpecsDir is tomo's documented planning-specs location
// (settings.ArtifactDir default).
const DefaultSpecsDir = "01_planning/specs"

// Report describes what Export did.
type Report struct {
	Change string
	Dest   string
	Copied []string
}

// Export copies root/specs/<change>/ into target/<specsDir>/<change>/,
// refusing change names that could escape the specs tree. An existing non-empty
// destination is refused unless force is set, which overwrites same-named files.
func Export(root, change, target, specsDir string, force bool) (Report, error) {
	rep := Report{Change: change}
	if change == "" || strings.ContainsAny(change, `/\`) || change == "." || change == ".." {
		return rep, fmt.Errorf("zumen: invalid change name %q", change)
	}
	if specsDir == "" {
		specsDir = DefaultSpecsDir
	}
	src := filepath.Join(state.SpecsDir(root), change)
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return rep, fmt.Errorf("zumen: change %q not found under %s", change, state.SpecsDir(root))
	}
	dst := filepath.Join(target, filepath.FromSlash(specsDir), change)
	rep.Dest = dst
	if !force {
		if entries, err := os.ReadDir(dst); err == nil && len(entries) > 0 {
			return rep, fmt.Errorf("zumen: %s already exists (pass --force to overwrite)", dst)
		}
	}
	err = filepath.Walk(src, func(p string, d os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() == ".gitkeep" {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := copyFile(p, out); err != nil {
			return err
		}
		rep.Copied = append(rep.Copied, rel)
		return nil
	})
	if err != nil {
		return rep, err
	}
	if len(rep.Copied) == 0 {
		return rep, fmt.Errorf("zumen: change %q has no files to export", change)
	}
	return rep, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
