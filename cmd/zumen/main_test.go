package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// silence discards stdout while fn runs.
func silence(t *testing.T, fn func() int) int {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { io.Copy(io.Discard, r); close(done) }()
	orig := os.Stdout
	os.Stdout = w
	code := fn()
	os.Stdout = orig
	w.Close()
	<-done
	return code
}

func TestUpdateHelpLeavesManifest(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	if code := silence(t, func() int { return run([]string{"init", dir, "--no-git"}) }); code != 0 {
		t.Fatalf("init = %d", code)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	manifest := filepath.Join(".zumen", "manifest.json")
	// An edited manifest would be rewritten by a real update.
	if err := os.WriteFile(manifest, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--help", "-h"} {
		if code := silence(t, func() int { return run([]string{"update", flag}) }); code != 0 {
			t.Fatalf("update %s = %d, want 0", flag, code)
		}
		got, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, []byte("{}\n")) {
			t.Fatalf("update %s rewrote the manifest: %q", flag, got)
		}
	}
}

// capture runs fn and returns its exit code and what it printed to stdout.
func capture(t *testing.T, fn func() int) (int, string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	done := make(chan struct{})
	go func() { io.Copy(&out, r); close(done) }()
	orig := os.Stdout
	os.Stdout = w
	code := fn()
	os.Stdout = orig
	w.Close()
	<-done
	return code, out.String()
}

// AC5: update prints that AGENTS.md was merged, and counts it.
func TestUpdatePrintsMerged(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	if code := silence(t, func() int { return run([]string{"init", dir, "--no-git"}) }); code != 0 {
		t.Fatalf("init = %d", code)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	data, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), "- <FILL IN: the target systems' stacks, or \"greenfield — to be decided in architecture\">", "- Go 1.22", 1) + "\n## Team notes\n\nkeep me\n"
	edited = strings.Replace(edited, "## Handoff\n", "## Handoff\n\nstale\n", 1)
	if err := os.WriteFile("AGENTS.md", []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := capture(t, func() int { return run([]string{"update"}) })
	if code != 0 {
		t.Fatalf("update = %d", code)
	}
	if !strings.Contains(out, "merged AGENTS.md (zumen's ## sections updated; your sections and the text under the title kept)") {
		t.Errorf("no merged line:\n%s", out)
	}
	if !strings.Contains(out, "(0 updated, 0 skipped, 0 unchanged, 1 merged)") && !strings.Contains(out, ", 1 merged)") {
		t.Errorf("summary doesn't count the merge:\n%s", out)
	}
	if strings.Contains(out, "skipped (edited) AGENTS.md") {
		t.Errorf("a merged file was also reported skipped:\n%s", out)
	}

	if err := os.WriteFile("AGENTS.md", []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, out = capture(t, func() int { return run([]string{"update"}) })
	if !strings.Contains(out, "skipped (edited) AGENTS.md: can't merge: no ## headings") {
		t.Errorf("no refusal line:\n%s", out)
	}
}
