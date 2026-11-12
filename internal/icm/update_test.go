package icm

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func scaffoldWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := Scaffold(dir, Options{NoGit: true}, ""); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestUpdateSkipsEditedAndRestoresMissing(t *testing.T) {
	dir := scaffoldWorkspace(t)
	agents := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Skipped) != 1 || rep.Skipped[0] != "AGENTS.md" {
		t.Fatalf("skipped = %v", rep.Skipped)
	}
	if len(rep.Updated) != 1 || rep.Updated[0] != "README.md" {
		t.Fatalf("updated = %v", rep.Updated)
	}
	if got, _ := os.ReadFile(agents); string(got) != "mine" {
		t.Fatalf("edit clobbered: %q", got)
	}
}

func TestUpdateReplacesUneditedFileFromOlderVersion(t *testing.T) {
	dir := scaffoldWorkspace(t)
	readme := filepath.Join(dir, "README.md")
	old := []byte("older scaffold")
	if err := os.WriteFile(readme, old, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.Files["README.md"] = hashBytes(old)
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Updated) != 1 || rep.Updated[0] != "README.md" {
		t.Fatalf("updated = %v", rep.Updated)
	}
	if got, _ := os.ReadFile(readme); string(got) == string(old) {
		t.Fatal("README.md was not refreshed")
	}
}

// A deleted .zumen/ must not cost the user their edits.
func TestUpdateRebuildsMissingManifestWithoutClobbering(t *testing.T) {
	dir := scaffoldWorkspace(t)
	if err := os.RemoveAll(filepath.Join(dir, WorkspaceDir)); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Skipped) != 1 || rep.Skipped[0] != "AGENTS.md" || len(rep.Updated) != 0 {
		t.Fatalf("rep = %+v", rep)
	}
	if got, _ := os.ReadFile(agents); string(got) != "mine" {
		t.Fatalf("edit clobbered: %q", got)
	}
	if _, err := ReadManifest(dir); err != nil {
		t.Fatalf("manifest not rebuilt: %v", err)
	}
}

func TestUpdateRefusesNonWorkspace(t *testing.T) {
	dir := t.TempDir()
	if _, err := Update(dir); err == nil || !strings.Contains(err.Error(), "not a zumen workspace") {
		t.Fatalf("want not a zumen workspace, got %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("wrote into a non-workspace: %v", entries)
	}
}

func TestScaffoldSeedsStateAtRootAndIgnoresOnlyActive(t *testing.T) {
	dir := scaffoldWorkspace(t)
	for _, rel := range []string{"STATE.json", "LOG.md", ".gitattributes", ".zumen/settings.json", ".zumen/manifest.json"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s not seeded: %v", rel, err)
		}
	}
	ignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(ignore), "\n")
	var active, whole bool
	for _, l := range lines {
		active = active || l == ".zumen/active"
		whole = whole || l == ".zumen/"
	}
	if !active || whole {
		t.Fatalf(".gitignore must ignore only .zumen/active:\n%s", ignore)
	}
}

// writeV1 lays out a v1 workspace by hand, its manifest recording each file as written.
func writeV1(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	m := Manifest{Version: "v0.2.0", Files: map[string]string{}}
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		m.Files[rel] = hashBytes([]byte(body))
	}
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMovesV1Rooms(t *testing.T) {
	dir := t.TempDir()
	writeV1(t, dir, map[string]string{
		"00_intake/context.md":                      "old intake",
		"01_explore/context.md":                     "old explore",
		"01_explore/findings/.gitkeep":              "",
		"02_requirements/context.md":                "old requirements",
		"02_requirements/templates/requirements.md": "old template",
		"03_design/context.md":                      "old design",
		"03_design/adr/.gitkeep":                    "",
		"03_design/templates/design.md":             "old design template",
		"04_plan/context.md":                        "old plan",
		"04_plan/templates/tasks.md":                "old tasks template",
		"AGENTS.md":                                 "old agents",
	})
	// Things a user made or changed in v1.
	for rel, body := range map[string]string{
		"03_design/adr/0001-use-go.md":        "a decision",
		"01_explore/findings/1-thing.md":      "findings",
		"02_requirements/personas/analyst.md": "my analyst",
	} {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	settingsPath := filepath.Join(dir, ".zumen", "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"adrDir": "03_design/adr", "gates": "block"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, old := range []string{"01_explore", "02_requirements", "03_design", "04_plan"} {
		if _, err := os.Stat(filepath.Join(dir, old)); !os.IsNotExist(err) {
			t.Errorf("%s still exists: %v", old, err)
		}
	}
	for rel, want := range map[string]string{
		"04_architecture/adr/0001-use-go.md": "a decision",
		"02_explore/findings/1-thing.md":     "findings",
	} {
		if got, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel))); string(got) != want {
			t.Errorf("%s = %q", rel, got)
		}
	}
	// Unedited managed files are refreshed, because the manifest followed the move.
	current, err := os.ReadFile(filepath.Join(dir, "03_spec", "context.md"))
	if err != nil || !strings.Contains(string(current), "03_spec") {
		t.Errorf("03_spec/context.md not refreshed: %q, %v", current, err)
	}
	// A file v1 didn't record, now at a managed path, is treated as edited.
	if got, _ := os.ReadFile(filepath.Join(dir, "03_spec", "personas", "analyst.md")); string(got) != "my analyst" {
		t.Errorf("edited persona clobbered: %q", got)
	}
	found := false
	for _, rel := range rep.Skipped {
		found = found || rel == "03_spec/personas/analyst.md"
	}
	if !found {
		t.Errorf("edited persona not reported: %v", rep.Skipped)
	}
	// Unedited files v2 dropped are gone.
	for _, rel := range []string{"05_breakdown/templates/tasks.md", "04_architecture/templates/design.md", "03_spec/templates/requirements.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s should be gone: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "05_breakdown", "templates", "ticket.md")); err != nil {
		t.Errorf("new template missing: %v", err)
	}
	if got, _ := os.ReadFile(settingsPath); !strings.Contains(string(got), `"adrDir": "04_architecture/adr"`) {
		t.Errorf("settings = %s", got)
	}
	// Running it again changes nothing about the layout.
	if moved, err := MigrateRooms(dir); err != nil || len(moved) != 0 {
		t.Errorf("second migration = %v, %v", moved, err)
	}
}

func TestMigrateRoomsKeepsEditedOrphansAndCustomSettings(t *testing.T) {
	dir := t.TempDir()
	writeV1(t, dir, map[string]string{"00_intake/context.md": "x", "04_plan/templates/tasks.md": "old"})
	if err := os.WriteFile(filepath.Join(dir, "04_plan", "templates", "tasks.md"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(dir, ".zumen", "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"adrDir": "decisions"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateRooms(dir); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "05_breakdown", "templates", "tasks.md")); string(got) != "mine" {
		t.Errorf("edited file lost: %q", got)
	}
	if got, _ := os.ReadFile(settingsPath); !strings.Contains(string(got), "decisions") {
		t.Errorf("custom adrDir changed: %s", got)
	}
}

func TestMigrateRoomsLeavesARoomWhoseNameIsTaken(t *testing.T) {
	dir := t.TempDir()
	for _, room := range []string{"04_plan", "05_breakdown"} {
		if err := os.MkdirAll(filepath.Join(dir, room), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	moved, err := MigrateRooms(dir)
	if err != nil || len(moved) != 0 {
		t.Fatalf("moved = %v, %v", moved, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "04_plan")); err != nil {
		t.Fatal(err)
	}
}

func TestSyncFileEditsSurviveUpdate(t *testing.T) {
	dir := scaffoldWorkspace(t)
	sync := filepath.Join(dir, "06_publish", "sync.md")
	if err := os.WriteFile(sync, []byte("my instructions"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(dir); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(sync); string(got) != "my instructions" {
		t.Fatalf("sync.md clobbered: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "06_publish", "sync-examples", "github.md")); err != nil {
		t.Fatal(err)
	}
}

// editAgents is the scaffold's AGENTS.md with the Tech stack section filled in,
// a section of the user's own added, and one zumen section gone stale.
func editAgents(t *testing.T, text string) string {
	t.Helper()
	doc := parseAgents(text)
	var b strings.Builder
	b.WriteString(strings.Join(doc.preamble, "\n") + "\n")
	for _, sec := range doc.sections {
		switch sec.heading {
		case "Tech stack & standards":
			b.WriteString("## Tech stack & standards\n\n- Go 1.22, Postgres\n\n")
		case "Handoff":
			b.WriteString("## Handoff\n\nstale handoff text\n\n")
		default:
			b.WriteString(strings.Join(sec.body, "\n") + "\n\n")
		}
	}
	b.WriteString("## Team notes\n\nkeep me\n")
	return b.String()
}

func headings(text string) []string {
	var out []string
	for _, sec := range parseAgents(text).sections {
		out = append(out, sec.heading)
	}
	return out
}

// AC5, AC7: an edited AGENTS.md gets zumen's sections back, and a second update
// changes nothing.
func TestUpdateMergesEditedAgents(t *testing.T) {
	dir := scaffoldWorkspace(t)
	path := filepath.Join(dir, "AGENTS.md")
	real := scaffoldAgents(t)
	if err := os.WriteFile(path, []byte(editAgents(t, real)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Merged) != 1 || rep.Merged[0] != "AGENTS.md" || len(rep.Skipped) != 0 || len(rep.NotMerged) != 0 {
		t.Fatalf("rep = %+v, want AGENTS.md merged and nothing skipped", rep)
	}
	got, _ := os.ReadFile(path)
	text := string(got)
	if strings.Contains(text, "stale handoff text") {
		t.Error("a zumen section wasn't refreshed")
	}
	for _, keep := range []string{"- Go 1.22, Postgres", "## Team notes\n\nkeep me\n"} {
		if !strings.Contains(text, keep) {
			t.Errorf("lost the user's %q", keep)
		}
	}
	if !strings.HasPrefix(text, strings.Join(parseAgents(real).preamble, "\n")) {
		t.Error("the title and intro changed")
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("AGENTS.md mode = %v, %v; want the user's 0640 kept", info.Mode().Perm(), err)
	}

	rep, err = Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Merged) != 0 || len(rep.Skipped) != 0 || !strings.Contains(strings.Join(rep.Unchanged, " "), "AGENTS.md") {
		t.Fatalf("second update rep = %+v, want AGENTS.md unchanged", rep)
	}
	if again, _ := os.ReadFile(path); string(again) != text {
		t.Error("the second update changed AGENTS.md")
	}
}

// AC5: a workspace whose AGENTS.md came from v0.7.0 or v0.8.0 and was edited.
func TestUpdateMergesFromOlderScaffold(t *testing.T) {
	real := scaffoldAgents(t)
	for _, version := range []string{"v0.7.0", "v0.8.0"} {
		fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "agents", version+"-AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		dir := scaffoldWorkspace(t)
		path := filepath.Join(dir, "AGENTS.md")
		if err := os.WriteFile(path, []byte(editAgents(t, string(fixture))), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := Update(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(rep.Merged) != 1 || len(rep.Skipped) != 0 {
			t.Fatalf("%s: rep = %+v", version, rep)
		}
		got, _ := os.ReadFile(path)
		merged, want := parseAgents(string(got)), parseAgents(real)
		wantBy := map[string]string{}
		for _, sec := range want.sections {
			wantBy[sec.heading] = strings.Join(sec.body, "\n")
		}
		for _, sec := range merged.sections {
			text := strings.Join(sec.body, "\n")
			switch sec.heading {
			case "Tech stack & standards":
				if !strings.Contains(text, "Go 1.22, Postgres") {
					t.Errorf("%s: Tech stack lost", version)
				}
			case "Team notes":
				if !strings.Contains(text, "keep me") {
					t.Errorf("%s: Team notes lost", version)
				}
			default:
				if text != wantBy[sec.heading] {
					t.Errorf("%s: %q doesn't match the current scaffold", version, sec.heading)
				}
			}
		}
		if want := append(headings(real), "Team notes"); len(headings(string(got))) != len(want) {
			t.Errorf("%s: headings = %v, want %d sections", version, headings(string(got)), len(want))
		}
	}
}

// AC6: a file that can't be merged is left alone, and the report says why.
func TestUpdateRefusesUnmergeableAgents(t *testing.T) {
	real := scaffoldAgents(t)
	for _, tc := range []struct{ name, content, reason string }{
		{"no ## headings", "mine", "no ## headings"},
		{"a duplicated zumen heading", real + "\n## Handoff\n\nagain\n", `duplicated heading "Handoff"`},
		{"an unclosed fence in a zumen section", strings.Replace(editAgents(t, real), "## Tech stack & standards", "Example:\n```sh\nzumen status\n\n## Tech stack & standards", 1), "unclosed code fence"},
	} {
		dir := scaffoldWorkspace(t)
		path := filepath.Join(dir, "AGENTS.md")
		if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := Update(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(rep.Skipped) != 1 || rep.Skipped[0] != "AGENTS.md" || rep.NotMerged["AGENTS.md"] != tc.reason || len(rep.Merged) != 0 {
			t.Errorf("%s: rep = %+v, want AGENTS.md skipped with %q", tc.name, rep, tc.reason)
		}
		if got, _ := os.ReadFile(path); string(got) != tc.content {
			t.Errorf("%s: the file changed", tc.name)
		}
	}
}

// AC8: a file that's still the previous scaffold is replaced as before, not merged.
func TestUpdateReplacesUneditedAgents(t *testing.T) {
	dir := scaffoldWorkspace(t)
	path := filepath.Join(dir, "AGENTS.md")
	old, err := os.ReadFile(filepath.Join("..", "..", "testdata", "agents", "v0.7.0-AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.Files["AGENTS.md"] = hashBytes(old)
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Merged) != 0 || len(rep.Updated) != 1 || rep.Updated[0] != "AGENTS.md" {
		t.Fatalf("rep = %+v, want AGENTS.md updated, not merged", rep)
	}
	if got, _ := os.ReadFile(path); string(got) != scaffoldAgents(t) {
		t.Error("AGENTS.md isn't the current scaffold")
	}
}

// AC1, AC9: init creates the references guide, and update adds it to an older
// workspace without touching any user file or creating the user's index.
func TestUpdateAddsReferencesWithoutTouchingUserFiles(t *testing.T) {
	dir := scaffoldWorkspace(t)
	guide := filepath.Join(dir, "references", "context.md")
	if _, err := os.Stat(guide); err != nil {
		t.Fatalf("init didn't create references/context.md: %v", err)
	}
	// Make it look like a workspace from before the room existed.
	if err := os.RemoveAll(filepath.Join(dir, "references")); err != nil {
		t.Fatal(err)
	}
	user := map[string]string{
		"00_intake/1.md":                "an intake",
		"specs/1-x/functional-spec.md":  "a spec",
		"01_research/1-x/notes.md":      "research",
		"02_explore/1-x/findings.md":    "findings",
		"04_architecture/adr/0001-x.md": "an adr",
		"specs/1-x/tickets/S-0001.md":   "a ticket",
		"specs/1-x/README.md":           "an overview",
	}
	for rel, content := range user {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(rep.Updated, "references/context.md") {
		t.Fatalf("updated = %v, want references/context.md", rep.Updated)
	}
	if _, err := os.Stat(guide); err != nil {
		t.Fatalf("references/context.md wasn't added: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "references", "index.md")); err == nil {
		t.Error("update created the user's index.md")
	}
	for rel, content := range user {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil || string(got) != content {
			t.Errorf("%s changed: %q, %v", rel, got, err)
		}
	}
}

// AC1: update adds the ideas room to an older workspace without touching a user file.
func TestUpdateAddsIdeasWithoutTouchingUserFiles(t *testing.T) {
	dir := scaffoldWorkspace(t)
	ideaFiles := []string{"ideas/context.md", "ideas/personas/facilitator.md", "ideas/templates/summary.md", "ideas/templates/log-entry.md"}
	for _, rel := range ideaFiles {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("init didn't create %s: %v", rel, err)
		}
	}
	if err := os.RemoveAll(filepath.Join(dir, "ideas")); err != nil {
		t.Fatal(err)
	}
	user := map[string]string{
		"00_intake/1.md":                "an intake",
		"specs/1-x/functional-spec.md":  "a spec",
		"references/index.md":           "my index",
		"references/notes.md":           "a note",
		"04_architecture/adr/0001-x.md": "an adr",
	}
	for rel, content := range user {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range ideaFiles {
		if !slices.Contains(rep.Updated, rel) {
			t.Errorf("updated = %v, want %s", rep.Updated, rel)
		}
	}
	for rel, content := range user {
		if got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel))); err != nil || string(got) != content {
			t.Errorf("%s changed: %q, %v", rel, got, err)
		}
	}
}

// AC17: a workspace whose AGENTS.md came from v0.8.0 and was edited gets the
// "Work an idea" row and step 0 through the section merge.
func TestUpdateDeliversTheIdeasRowThroughTheMerge(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "agents", "v0.8.0-AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	dir := scaffoldWorkspace(t)
	path := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(path, []byte(editAgents(t, string(fixture))), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Merged) != 1 || len(rep.Skipped) != 0 {
		t.Fatalf("rep = %+v, want AGENTS.md merged", rep)
	}
	got, _ := os.ReadFile(path)
	text := string(got)
	for _, want := range []string{
		"| Work an idea | `ideas/` | facilitator |",
		"0. **Work an idea first** (optional)",
		"`zumen idea promote",
		"- Go 1.22, Postgres",
		"## Team notes\n\nkeep me\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("merged AGENTS.md lacks %q", want)
		}
	}
	if strings.Index(text, "| Work an idea |") > strings.Index(text, "| Define the base source |") {
		t.Error("the Work an idea row isn't first in the routing table")
	}
}
