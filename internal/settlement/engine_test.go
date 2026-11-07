package settlement

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/model"
)

func event(rev int64, tip string, confirmed bool, txs ...model.Tx) model.Event {
	e := model.Event{ID: fmt.Sprintf("event-%d", rev), Revision: rev, Tip: tip, Height: 1, Blocks: []string{"genesis", tip}, Transactions: txs}
	for j := range e.Transactions {
		if confirmed {
			e.Transactions[j].Status = "confirmed"
			e.Transactions[j].Height = 1
			e.Transactions[j].BlockHash = tip
			e.Transactions[j].Confirmations = 1
		} else {
			e.Transactions[j].Status = "mempool"
			e.Transactions[j].Height = -1
		}
	}
	return e
}
func payment(id string, amount model.Sats) model.Tx {
	t := model.Tx{TxID: id, Vin: []model.Input{{TxID: "funding", Vout: 0}}, Vout: []model.Output{{N: 0, Value: amount}}}
	t.Vout[0].Script.Address = "merchant"
	return t
}
func TestJournalReorgRestartReplay(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "db")
	e, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { e.Close() }()
	i := model.Invoice{ID: "invoice", Address: "merchant", Expected: 50000, Required: 1}
	if err = e.Create(ctx, i); err != nil {
		t.Fatal(err)
	}
	a := payment("a", 50000)
	b := payment("b", 30000)
	history := []model.Event{event(1, "a0", false, a), event(2, "a1", true, a), event(3, "b1", true, b), event(4, "none", true)}
	for j, v := range history {
		if err = e.Apply(ctx, v); err != nil {
			t.Fatal(err)
		}
		if err = e.Apply(ctx, v); err != nil {
			t.Fatal(err)
		}
		if j == 1 {
			e.Close()
			e, err = Open(path)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	s, err := e.State(ctx, i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s.Balance != 0 || s.Gross != 80000 || s.Reversed != 80000 || len(s.Entries) != 4 {
		t.Fatalf("bad journal %+v", s)
	}
	for _, v := range history {
		if err = e.Apply(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	s, _ = e.State(ctx, i.ID)
	if s.Balance != 0 || len(s.Entries) != 4 {
		t.Fatal("stale replay changed state")
	}
	if _, err = e.DB.Exec("DELETE FROM settlement_entries"); err == nil {
		t.Fatal("journal deletion allowed")
	}
	if err = e.Apply(ctx, event(5, "a1", true, a)); err != nil {
		t.Fatal(err)
	}
	s, _ = e.State(ctx, i.ID)
	if s.Balance != 50000 || len(s.Entries) != 5 {
		t.Fatal("reconfirmed output not recredited")
	}
}
func TestThresholdIndependentPaymentsAndUnknown(t *testing.T) {
	ctx := context.Background()
	e, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	e.Create(ctx, model.Invoice{ID: "i", Address: "merchant", Expected: 50000, Required: 2})
	a := payment("a", 30000)
	b := payment("b", 20000)
	b.Vin[0].TxID = "other-funding"
	v := event(1, "one", true, a, b)
	if err = e.Apply(ctx, v); err != nil {
		t.Fatal(err)
	}
	s, _ := e.State(ctx, "i")
	if s.Balance != 0 || s.Pending != 50000 {
		t.Fatalf("premature credit %+v", s)
	}
	v = event(2, "two", true, a, b)
	v.Height = 2
	v.Blocks = []string{"genesis", "one", "two"}
	for j := range v.Transactions {
		v.Transactions[j].BlockHash = "one"
		v.Transactions[j].Confirmations = 2
	}
	if err = e.Apply(ctx, v); err != nil {
		t.Fatal(err)
	}
	s, _ = e.State(ctx, "i")
	if s.Balance != 50000 || len(s.Entries) != 2 {
		t.Fatalf("independent payments lost %+v", s)
	}
	v = event(3, "gone", false)
	if err = e.Apply(ctx, v); err != nil {
		t.Fatal(err)
	}
	s, _ = e.State(ctx, "i")
	if s.Transactions[0].Status != "unknown" {
		t.Fatal("disappearance falsely classified as replacement")
	}
}
func TestAtomicInvalidObservation(t *testing.T) {
	ctx := context.Background()
	e, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	e.Create(ctx, model.Invoice{ID: "i", Address: "merchant", Expected: 1, Required: 1})
	v := event(1, "tip", true, payment("a", 1))
	v.Transactions[0].BlockHash = "wrong"
	if e.Apply(ctx, v) == nil {
		t.Fatal("invalid chain context accepted")
	}
	s, _ := e.State(ctx, "i")
	if s.Balance != 0 || len(s.Transactions) != 0 {
		t.Fatal("partial commit")
	}
	if e.Apply(ctx, model.Event{ID: "missing-context"}) == nil {
		t.Fatal("context-free event accepted")
	}
}

func TestRedirectConflictDiscovery(t *testing.T) {
	ctx := context.Background()
	e, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err = e.Create(ctx, model.Invoice{ID: "i", Address: "merchant", Expected: 50000, Required: 1}); err != nil {
		t.Fatal(err)
	}
	a := payment("original", 50000)
	if err = e.Apply(ctx, event(1, "baseline", false, a)); err != nil {
		t.Fatal(err)
	}
	b := payment("redirected", 50000)
	b.Vout[0].Script.Address = "elsewhere"
	if err = e.Apply(ctx, event(2, "winning", true, b)); err != nil {
		t.Fatal(err)
	}
	s, err := e.State(ctx, "i")
	if err != nil {
		t.Fatal(err)
	}
	if s.Balance != 0 || s.Invoice.Status != "unpaid" || len(s.Transactions) != 2 {
		t.Fatalf("lost redirect evidence: %+v", s)
	}
	found := false
	for _, tx := range s.Transactions {
		if tx.TxID == "original" && tx.Status == "conflicted" {
			found = true
		}
	}
	if !found {
		t.Fatal("original shared-input conflict untracked")
	}
	var count int
	if err = e.DB.QueryRow("SELECT COUNT(*) FROM transaction_conflicts").Scan(&count); err != nil || count != 1 {
		t.Fatalf("conflict row %d %v", count, err)
	}
	v := event(2, "winning", true, b)
	v.ID = "collision"
	if e.Apply(ctx, v) == nil {
		t.Fatal("ambiguous event revision accepted")
	}
}
