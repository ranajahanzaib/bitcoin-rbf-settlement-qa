package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/invariant"
)

func TestReportRoundtrip(t *testing.T) {
	r := Report{RunID: "test", Scenario: "normal-confirmation", Started: time.Now().UTC(), ArtifactDir: t.TempDir(), Assertions: []invariant.Result{{ID: "INV-001", Passed: true}}}
	if err := Write(&r); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(r.ArtifactDir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got Report
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Passed() || got.Duration == "" || !strings.Contains(got.Markdown(), "PASS") {
		t.Fatalf("bad report %+v", got)
	}
}
