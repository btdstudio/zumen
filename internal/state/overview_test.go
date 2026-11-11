package state

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// overviewRoot is a local (vcs: none) workspace with two seeded changes;
// 1-auth is active.
func overviewRoot(t *testing.T, overview string) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	settings := `{"vcs": "none"`
	if overview != "" {
		settings += `, "overview": "` + overview + `"`
	}
	if err := os.MkdirAll(Workspace(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Workspace(root), "settings.json"), []byte(settings+"}"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"2-other", "1-auth"} {
		if _, err := SeedChange(root, name, false, nil); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func readmeAt(root, change string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(OverviewPath(change))))
	return string(data), err == nil
}

func TestRefreshHookIsWired(t *testing.T) {
	if store.Refresh == nil {
		t.Fatal("store.Refresh isn't set")
	}
	if s, err := storeAt(t.TempDir()); err != nil || s.Refresh == nil {
		t.Fatalf("storeAt's copy lost Refresh: %v", err)
	}
}

// A gate and a reopen regenerate the active change's README through kiso.
func TestTransitionsRegenerateOverview(t *testing.T) {
	root := overviewRoot(t, "")
	if _, ok := readmeAt(root, "1-auth"); ok {
		t.Fatal("seeding wrote a README")
	}
	if _, err := AdvanceGate(root, "intake", "approved", ""); err != nil {
		t.Fatal(err)
	}
	text, ok := readmeAt(root, "1-auth")
	if !ok || !strings.Contains(text, "| intake | approved |  |") {
		t.Fatalf("after the gate:\n%s", text)
	}
	if _, ok := readmeAt(root, "2-other"); ok {
		t.Fatal("an untouched change got a README")
	}
	if _, err := Reopen(root, "intake", "rework"); err != nil {
		t.Fatal(err)
	}
	if text, _ := readmeAt(root, "1-auth"); !strings.Contains(text, "| intake | draft |  |") {
		t.Fatalf("after the reopen:\n%s", text)
	}
}

func TestRefreshOverviewManualDoesNothing(t *testing.T) {
	root := overviewRoot(t, "manual")
	if _, err := AdvanceGate(root, "intake", "approved", ""); err != nil {
		t.Fatal(err)
	}
	written, err := RefreshOverview(root, "1-auth")
	if written != nil || err != nil {
		t.Fatalf("manual = %v, %v", written, err)
	}
	if _, ok := readmeAt(root, "1-auth"); ok {
		t.Fatal("manual wrote a README")
	}
}

func TestRefreshOverviewWritesOnlyChanges(t *testing.T) {
	root := overviewRoot(t, "auto")
	written, err := RefreshOverview(root, "2-other", "1-auth", "1-auth", "")
	if err != nil || !reflect.DeepEqual(written, []string{"specs/1-auth/README.md", "specs/2-other/README.md"}) {
		t.Fatalf("first = %v, %v", written, err)
	}
	if written, err := RefreshOverview(root, "1-auth", "2-other"); err != nil || written != nil {
		t.Fatalf("unchanged = %v, %v", written, err)
	}
	merged, existing, rel, err := Overview(root, "1-auth")
	if err != nil || string(merged) != string(existing) || rel != "specs/1-auth/README.md" {
		t.Fatalf("Overview disagrees with what RefreshOverview wrote: %v", err)
	}
}

// A malformed README is reported naming it and left alone; the others are
// still written.
func TestRefreshOverviewMalformedIsAnError(t *testing.T) {
	root := overviewRoot(t, "")
	bad := "<!-- zumen:narrative -->\n<!-- /zumen:narrative -->\n<!-- zumen:generated -->\n<!-- zumen:generated -->\n<!-- /zumen:generated -->\n"
	p := filepath.Join(root, filepath.FromSlash(OverviewPath("1-auth")))
	if err := os.WriteFile(p, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := RefreshOverview(root, "1-auth", "2-other")
	if err == nil || err.Error() != "zumen: specs/1-auth/README.md: duplicated marker <!-- zumen:generated --> (lines 3 and 4)" {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(written, []string{"specs/2-other/README.md"}) {
		t.Fatalf("written = %v", written)
	}
	if text, _ := readmeAt(root, "1-auth"); text != bad {
		t.Fatal("the malformed README changed")
	}
	// Through a gate it's a warning: the transition still succeeds.
	if st, err := AdvanceGate(root, "intake", "approved", ""); err != nil || st.Gates["intake"] != "approved" {
		t.Fatalf("gate = %+v, %v", st, err)
	}
	if text, _ := readmeAt(root, "1-auth"); text != bad {
		t.Fatal("the gate changed the malformed README")
	}
}

func TestRefreshOverviewBadSettings(t *testing.T) {
	root := overviewRoot(t, "")
	if err := os.WriteFile(filepath.Join(Workspace(root), "settings.json"), []byte(`{"vcs": "none", "overview": "sometimes"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshOverview(root, "1-auth"); err == nil || !strings.Contains(err.Error(), `invalid overview "sometimes"`) {
		t.Fatalf("err = %v", err)
	}
}

// A problem that already says "zumen: " isn't prefixed twice, and still unwraps.
func TestRefreshOverviewBadRepos(t *testing.T) {
	root := overviewRoot(t, "")
	if err := os.WriteFile(filepath.Join(root, "REPOS.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := RefreshOverview(root, "1-auth")
	if err == nil || !strings.HasPrefix(err.Error(), "zumen: specs/1-auth/README.md: cannot parse ") || strings.Count(err.Error(), "zumen: ") != 1 {
		t.Fatalf("err = %v", err)
	}
	inner := errors.New("zumen: boom")
	if wrapped := OverviewError("x", inner); wrapped.Error() != "zumen: x: boom" || !errors.Is(wrapped, inner) {
		t.Fatalf("OverviewError = %v", wrapped)
	}
}
