package icm

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden fixtures")

// TestGoldenManifest pins the scaffold's byte-identity: any edit to scaffold/
// changes a file hash and fails this test until the golden is regenerated.
func TestGoldenManifest(t *testing.T) {
	m, err := BuildManifest()
	if err != nil {
		t.Fatal(err)
	}
	m.Version = "dev" // normalize (release builds stamp a real version)
	golden := filepath.Join("..", "..", "testdata", "golden", "icm", "manifest.json")
	if *update {
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, '\n')
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v (run `go test ./internal/icm -update`)", err)
	}
	got, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if string(got) != string(want) {
		t.Fatalf("golden mismatch; run `go test ./internal/icm -update` to regenerate")
	}
}
