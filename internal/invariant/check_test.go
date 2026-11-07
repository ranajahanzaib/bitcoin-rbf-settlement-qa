package invariant

import (
	"testing"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/model"
)

// Deliberately faulty merchant states stay isolated in tests.
func TestDetectFaultyMerchants(t *testing.T) {
	for _, c := range []struct {
		name   string
		state  model.State
		oracle Evidence
		fails  string
	}{{"duplicate-credit", model.State{Invoice: model.Invoice{ID: "i", Expected: 50000, Required: 1, Status: "settled"}, Balance: 100000, Entries: []model.Entry{{ID: 1, Key: "first", Invoice: "i", Output: "tx:0", Delta: 50000}, {ID: 2, Key: "duplicate", Invoice: "i", Output: "tx:0", Delta: 50000}}, Revision: 1, Tip: "tip"}, Evidence{Tip: "tip", Amount: 50000, Eligible: map[string]int64{"tx:0": 50000}, Confirmed: map[string]int{"tx:0": 1}}, "INV-001"}, {"missing-reversal", model.State{Invoice: model.Invoice{ID: "i", Expected: 50000, Required: 1, Status: "settled"}, Balance: 50000, Entries: []model.Entry{{ID: 1, Key: "orphan-credit", Invoice: "i", Output: "orphan:0", Delta: 50000}}, Revision: 2, Tip: "new-tip"}, Evidence{Tip: "new-tip", Eligible: map[string]int64{}, Confirmed: map[string]int{}}, "INV-007"}} {
		t.Run(c.name, func(t *testing.T) {
			detected := false
			for _, r := range Check(c.state, c.oracle, c.name) {
				if r.ID == c.fails && !r.Passed {
					detected = true
				}
			}
			if !detected {
				t.Fatal("fault escaped detection")
			}
		})
	}
}

func TestSplitCreditsStillViolateOutputIdempotency(t *testing.T) {
	s := model.State{Invoice: model.Invoice{ID: "i", Expected: 50000, Required: 1, Status: "settled"}, Balance: 50000, Entries: []model.Entry{{ID: 1, Key: "half-1", Invoice: "i", Output: "tx:0", Delta: 25000}, {ID: 2, Key: "half-2", Invoice: "i", Output: "tx:0", Delta: 25000}}, Revision: 1, Tip: "tip"}
	o := Evidence{Tip: "tip", Amount: 50000, Eligible: map[string]int64{"tx:0": 50000}, Confirmed: map[string]int{"tx:0": 1}}
	for _, r := range Check(s, o, "split-credit") {
		if r.ID == "INV-001" {
			if r.Passed {
				t.Fatal("multiple effective credits for same output accepted")
			}
			return
		}
	}
	t.Fatal("missing invariant")
}
