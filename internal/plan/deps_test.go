package plan_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/zumen/internal/state"
)

// plan's specs folder is a constant, because plan can't import state. This
// fails if state's store ever moves the change folders.
func TestSpecsDirMatchesState(t *testing.T) {
	root := t.TempDir()
	if got, want := state.ChangeDir(root, "1-x"), filepath.Join(root, "specs", "1-x"); got != want {
		t.Fatalf("state.ChangeDir = %q, plan uses %q", got, want)
	}
}

// AC9: plan sits below state and cli in the import graph.
func TestNoStateOrCLIDependency(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	cmd := exec.Command("go", "list", "-deps", ".")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	deps := strings.Fields(string(out))
	if len(deps) == 0 {
		t.Fatal("go list printed nothing")
	}
	for _, d := range deps {
		if d == "github.com/btdstudio/zumen/internal/state" || d == "github.com/btdstudio/zumen/internal/cli" {
			t.Errorf("internal/plan depends on %s", d)
		}
	}
}
