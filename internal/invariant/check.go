package invariant

import (
	"context"
	"fmt"
	"strings"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/core"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/model"
)

type Result struct {
	ID         string `json:"id"`
	Passed     bool   `json:"passed"`
	Message    string `json:"message"`
	Expected   int64  `json:"expected_sats"`
	Actual     int64  `json:"actual_sats"`
	Checkpoint string `json:"checkpoint"`
}
type Evidence struct {
	Tip       string           `json:"tip"`
	Height    int              `json:"height"`
	Eligible  map[string]int64 `json:"eligible_outputs"`
	Confirmed map[string]int   `json:"confirmations"`
	Amount    int64            `json:"amount_sats"`
}

// Oracle independently walks active blocks; it never reads the merchant's observations or DB.
func Oracle(ctx context.Context, r *core.RPC, i model.Invoice) (o Evidence, err error) {
	o.Eligible = map[string]int64{}
	o.Confirmed = map[string]int{}
	var info struct {
		Chain  string `json:"chain"`
		Height int    `json:"blocks"`
		Tip    string `json:"bestblockhash"`
	}
	if err = r.Call(ctx, "getblockchaininfo", &info); err != nil {
		return
	}
	if info.Chain != "regtest" {
		err = fmt.Errorf("oracle refuses non-regtest chain")
		return
	}
	o.Tip = info.Tip
	o.Height = info.Height
	for h := 0; h <= o.Height; h++ {
		var hash string
		if err = r.Call(ctx, "getblockhash", &hash, h); err != nil {
			return
		}
		var b model.Block
		if err = r.Call(ctx, "getblock", &b, hash, 2); err != nil {
			return
		}
		for _, t := range b.Tx {
			for _, v := range t.Vout {
				if v.Script.Address == i.Address {
					key := fmt.Sprintf("%s:%d", t.TxID, v.N)
					depth := o.Height - h + 1
					o.Confirmed[key] = depth
					if depth >= i.Required {
						o.Eligible[key] = int64(v.Value)
						o.Amount += int64(v.Value)
					}
				}
			}
		}
	}
	var tip string
	if err = r.Call(ctx, "getbestblockhash", &tip); err != nil {
		return
	}
	if tip != o.Tip {
		err = fmt.Errorf("oracle tip changed during scan")
	}
	return
}
func Check(s model.State, o Evidence, checkpoint string) []Result {
	active := map[string]int64{}
	activeCount := map[string]int{}
	credits := map[int64]model.Entry{}
	reversed := map[int64]bool{}
	keys := map[string]bool{}
	ids := map[int64]bool{}
	duplicates := false
	validJournal := true
	var sum int64
	for _, e := range s.Entries {
		if keys[e.Key] || ids[e.ID] || e.ID <= 0 || e.Key == "" {
			duplicates = true
		}
		keys[e.Key] = true
		ids[e.ID] = true
		if e.Invoice != s.Invoice.ID {
			validJournal = false
		}
		sum += e.Delta
		if e.Delta > 0 {
			credits[e.ID] = e
		} else {
			c, ok := credits[e.Reversal]
			if !ok || c.Output != e.Output || e.Delta != -c.Delta || reversed[e.Reversal] {
				validJournal = false
			}
			reversed[e.Reversal] = true
		}
	}
	for id, e := range credits {
		if !reversed[id] {
			active[e.Output] += e.Delta
			activeCount[e.Output]++
			if activeCount[e.Output] > 1 {
				duplicates = true
			}
		}
	}
	canonical, amount, threshold, reversal := true, true, true, true
	for key, value := range active {
		if want, ok := o.Eligible[key]; !ok {
			canonical = false
			amount = false
			reversal = false
		} else if value != want {
			amount = false
			duplicates = true
		}
		if o.Confirmed[key] < s.Invoice.Required {
			threshold = false
		}
	}
	for key, value := range o.Eligible {
		if active[key] != value {
			amount = false
		}
	}
	net := s.Balance == o.Amount && sum == s.Balance
	status := "unpaid"
	if s.Pending > 0 {
		status = "observed"
	}
	if o.Amount > 0 {
		status = "partially_paid"
	}
	if o.Amount >= s.Invoice.Expected {
		status = "settled"
	}
	outstanding := s.Invoice.Expected - o.Amount
	if outstanding < 0 {
		outstanding = 0
	}
	checks := []struct {
		id, msg string
		ok      bool
	}{{"INV-001", "No duplicate effective credit", !duplicates}, {"INV-002", "Credits correspond to active-chain outputs", canonical}, {"INV-003", "Credited output amounts equal canonical amounts", amount}, {"INV-004", "Confirmation threshold enforced", threshold}, {"INV-005", "Conflicting versions cannot create additional finalized value", canonical && amount}, {"INV-006", "Redirected spends produce no merchant credit", o.Amount != 0 || s.Balance == 0 && sum == 0}, {"INV-007", "Orphaned credits reversed with valid compensating entries", reversal && validJournal}, {"INV-008", "Replay leaves canonical journal balance", net && validJournal}, {"INV-009", "Durable checkpoint agrees with authoritative chain", s.Tip == o.Tip && s.Revision > 0}, {"INV-010", "Invoice reconciles against independent canonical outputs", net && s.Outstanding == outstanding && s.Invoice.Status == status}}
	out := make([]Result, 0, len(checks))
	for _, c := range checks {
		msg := c.msg
		if !c.ok {
			msg = fmt.Sprintf("%s FAILED: %s; invoice=%s expected=%d actual=%d tip=%s outputs=%s", c.id, c.msg, s.Invoice.ID, o.Amount, s.Balance, o.Tip, strings.Join(outputKeys(active), ","))
		}
		out = append(out, Result{c.id, c.ok, msg, o.Amount, s.Balance, checkpoint})
	}
	return out
}
func outputKeys(m map[string]int64) []string {
	var s []string
	for k := range m {
		s = append(s, k)
	}
	return s
}
