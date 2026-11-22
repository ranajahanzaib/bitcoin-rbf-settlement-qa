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

func TestReportMarkdownFailuresContiguousTable(t *testing.T) {
	r := Report{
		RunID:       "test-failure",
		Scenario:    "duplicate-observer-events",
		Started:     time.Now().UTC(),
		ArtifactDir: t.TempDir(),
		Assertions: []invariant.Result{
			{ID: "INV-001", Checkpoint: "settled", Passed: false, Message: "No duplicate credit", Expected: 50000, Actual: 100000},
			{ID: "INV-002", Checkpoint: "settled", Passed: true, Message: "Valid outputs", Expected: 50000, Actual: 100000},
		},
	}
	md := r.Markdown()
	if !strings.Contains(md, "**FAIL**") {
		t.Fatalf("expected FAIL in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "### Violations") {
		t.Fatalf("expected ### Violations section, got:\n%s", md)
	}
	if !strings.Contains(md, "| settled | INV-001 | FAIL | 50000 | 100000 |\n| settled | INV-002 | PASS | 50000 | 100000 |") {
		t.Fatalf("expected contiguous table rows without interleaved message, got:\n%s", md)
	}
}
