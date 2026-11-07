package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/core"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/invariant"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/model"
)

type Snapshot struct {
	Name   string             `json:"checkpoint"`
	State  model.State        `json:"merchant"`
	Oracle invariant.Evidence `json:"oracle"`
}
type Report struct {
	Scenario        string             `json:"scenario"`
	Adapter         string             `json:"adapter"`
	ScenarioVersion int                `json:"scenario_version"`
	RunID           string             `json:"run_id"`
	Started         time.Time          `json:"started_utc"`
	Finished        time.Time          `json:"finished_utc"`
	Duration        string             `json:"duration"`
	CoreVersion     int                `json:"core_version"`
	Network         string             `json:"network"`
	Policy          map[string]any     `json:"policy"`
	Nodes           []string           `json:"nodes"`
	Transactions    []model.Tx         `json:"transactions"`
	Acceptance      []core.Acceptance  `json:"mempool_acceptance"`
	Events          []model.Event      `json:"events"`
	Snapshots       []Snapshot         `json:"snapshots"`
	Assertions      []invariant.Result `json:"invariants"`
	Failure         string             `json:"failure,omitempty"`
	ExitCode        int                `json:"exit_code"`
	ArtifactDir     string             `json:"artifact_directory"`
}

func (r Report) Passed() bool {
	if r.ExitCode != 0 || r.Failure != "" || len(r.Assertions) == 0 {
		return false
	}
	for _, a := range r.Assertions {
		if !a.Passed {
			return false
		}
	}
	return true
}
func (r Report) Markdown() string {
	adapter := r.Adapter
	if adapter == "" {
		adapter = "reference"
	}
	quotedAdapter := "'" + strings.ReplaceAll(adapter, "'", "'\"'\"'") + "'"
	var b strings.Builder
	fmt.Fprintf(&b, "# Bitcoin RBF Settlement QA\n\nScenario: `%s` (v%d)\n\nRun: `%s`\n\nCore: %d · Network: %s · Duration: %s\n\n", r.Scenario, r.ScenarioVersion, r.RunID, r.CoreVersion, r.Network, r.Duration)
	if r.Passed() {
		b.WriteString("**PASS — Settlement accounting consistent**\n\n")
	} else {
		b.WriteString("**FAIL**\n\n")
	}
	if r.Failure != "" {
		fmt.Fprintf(&b, "Execution failure: %s\n\n", r.Failure)
	}
	b.WriteString("| Checkpoint | Invariant | Result | Expected sats | Actual sats |\n|---|---|---|---:|---:|\n")
	for _, a := range r.Assertions {
		status := "PASS"
		if !a.Passed {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d |\n", a.Checkpoint, a.ID, status, a.Expected, a.Actual)
		if !a.Passed {
			fmt.Fprintf(&b, "\n%s\n\n", a.Message)
		}
	}
	b.WriteString("\n## Transaction timeline\n\n")
	for j, e := range r.Events {
		fmt.Fprintf(&b, "%d. Revision %d: tip `%s`, height %d, %d disconnected blocks.\n", j+1, e.Revision, e.Tip, e.Height, len(e.Disconnected))
	}
	b.WriteString("\n## Merchant snapshots and journal\n\n")
	for _, s := range r.Snapshots {
		fmt.Fprintf(&b, "- %s: expected %d sats; actual %d sats; status %s; gross credits %d; reversals %d.\n", s.Name, s.Oracle.Amount, s.State.Balance, s.State.Invoice.Status, s.State.Gross, s.State.Reversed)
		for _, e := range s.State.Entries {
			fmt.Fprintf(&b, "  - Entry %d: `%s`, delta %d, reversal of %d.\n", e.ID, e.Output, e.Delta, e.Reversal)
		}
	}
	fmt.Fprintf(&b, "\n## Reproduce\n\n`rbfqa run %s --adapter %s --keep-artifacts`\n\nSee report.json for exact chain, transaction, acceptance, and accounting evidence. Offline accounting replay: `rbfqa replay %s`. Use the same test adapter implementation and confirmation policy recorded in the snapshots. Raw transactions are regtest-only and require the original funding chain.\n", r.Scenario, quotedAdapter, filepath.Join(r.ArtifactDir, "report.json"))
	return b.String()
}
func Write(r *Report) error {
	r.Finished = time.Now().UTC()
	r.Duration = r.Finished.Sub(r.Started).String()
	if err := os.MkdirAll(r.ArtifactDir, 0700); err != nil {
		return err
	}
	for name, v := range map[string]any{"report.json": r, "events.json": r.Events, "transactions.json": r.Transactions} {
		b, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(r.ArtifactDir, name), append(b, '\n'), 0600); e != nil {
			return e
		}
	}
	return os.WriteFile(filepath.Join(r.ArtifactDir, "report.md"), []byte(r.Markdown()), 0600)
}
