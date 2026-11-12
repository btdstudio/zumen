package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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
