package icm

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/zumen"
)

// localFiles are the machine-local files zumen's .gitignore must keep out of git.
var localFiles = []string{".zumen/active", ".zumen/paths.json"}

// isolateGit runs git with no user or system configuration: no global or system
// config and no default excludes file ($XDG_CONFIG_HOME/git/ignore or
// ~/.config/git/ignore), so only the workspace's own .gitignore can hide a file.
// HOME and XDG_CONFIG_HOME also cover git older than 2.32, which ignores
// GIT_CONFIG_GLOBAL. XDG_CONFIG_HOME also leaves zumen's global settings empty.
func isolateGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func runGitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// writeLocalFiles writes the two local files and a control file that git must
// still see, so an empty `git status` can't pass for ignored.
func writeLocalFiles(t *testing.T, ws string) {
	t.Helper()
	for rel, body := range map[string]string{
		".zumen/active":     "1-x\n",
		".zumen/paths.json": `{"app": "/abs/path/app"}` + "\n",
		"control.md":        "visible\n",
	} {
		p := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// requireIgnored checks `git status` in repo: the control file is listed and
// neither local file is. prefix is the workspace's path inside repo ("" or "ws/").
func requireIgnored(t *testing.T, repo, prefix string) {
	t.Helper()
	status := runGitIn(t, repo, "status", "--porcelain", "--untracked-files=all")
	if !strings.Contains(status, "?? "+prefix+"control.md") {
		t.Fatalf("control file not listed, so the check proves nothing:\n%s", status)
	}
	for _, rel := range localFiles {
		if strings.Contains(status, prefix+rel) {
			t.Errorf("%s%s is not ignored:\n%s", prefix, rel, status)
		}
	}
}

func scaffoldLocal(t *testing.T, ws string) {
	t.Helper()
	if _, err := Scaffold(ws, Options{NoGit: true}, "none"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".git")); !os.IsNotExist(err) {
		t.Fatalf("local scaffold made %s/.git (err = %v)", ws, err)
	}
}

// N-0002 AC5: a local workspace inside an enclosing repo keeps its local files out
// of that repo.
func TestLocalFilesIgnoredInEnclosingRepo(t *testing.T) {
	isolateGit(t)
	parent := t.TempDir()
	runGitIn(t, parent, "init", "-q")
	ws := filepath.Join(parent, "ws")
	scaffoldLocal(t, ws)
	writeLocalFiles(t, ws)
	requireIgnored(t, parent, "ws/")
}

// N-0002 AC6: after `git init` in a local workspace, its local files are already
// ignored, before any `zumen update`.
func TestLocalFilesIgnoredAfterGitInit(t *testing.T) {
	isolateGit(t)
	ws := t.TempDir()
	scaffoldLocal(t, ws)
	writeLocalFiles(t, ws)
	runGitIn(t, ws, "init", "-q")
	requireIgnored(t, ws, "")
}

// N-0002 R5: the embedded .gitignore lists both local files; the protection is
// only as good as this file.
func TestScaffoldGitignoreListsLocalFiles(t *testing.T) {
	data, err := fs.ReadFile(zumen.ScaffoldFS, "scaffold/.gitignore")
	if err != nil {
		t.Fatal(err)
	}
	lines := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		lines[strings.TrimSpace(line)] = true
	}
	for _, rel := range localFiles {
		if !lines[rel] {
			t.Errorf("scaffold/.gitignore lacks %s", rel)
		}
	}
}
