package core

import (
	"context"
	"fmt"
	"sort"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/model"
)

type UTXO struct {
	TxID   string     `json:"txid"`
	Vout   int        `json:"vout"`
	Amount model.Sats `json:"amount"`
}
type Acceptance struct {
	TxID         string `json:"txid"`
	Allowed      bool   `json:"allowed"`
	RejectReason string `json:"reject-reason,omitempty"`
	Details      string `json:"reject-details,omitempty"`
	Vsize        int    `json:"vsize,omitempty"`
}

func (n *Node) Funding(ctx context.Context) (UTXO, error) {
	var us []UTXO
	if e := n.Wallet.Call(ctx, "listunspent", &us, 1); e != nil {
		return UTXO{}, e
	}
	return selectFunding(us)
}

func selectFunding(us []UTXO) (UTXO, error) {
	sort.Slice(us, func(i, j int) bool {
		if us[i].TxID == us[j].TxID {
			return us[i].Vout < us[j].Vout
		}
		return us[i].TxID < us[j].TxID
	})
	// Exclude small merchant outputs created by earlier variants. Every built-in
	// transaction needs at most 90,000 sats; keep ample explicit fee headroom.
	for _, u := range us {
		if u.Amount >= 1_000_000 {
			return u, nil
		}
	}
	return UTXO{}, fmt.Errorf("no confirmed funding output with at least 1,000,000 sats")
}
func (n *Node) Build(ctx context.Context, u UTXO, dest, change string, amount, fee int64, sequence uint32) (model.Tx, error) {
	var t model.Tx
	if amount <= 0 || fee <= 0 || int64(u.Amount) <= amount+fee {
		return t, fmt.Errorf("invalid funding arithmetic")
	}
	outputs := []map[string]any{{dest: model.Sats(amount).BTC()}, {change: model.Sats(int64(u.Amount) - amount - fee).BTC()}}
	var raw string
	if e := n.Wallet.Call(ctx, "createrawtransaction", &raw, []model.Input{{TxID: u.TxID, Vout: u.Vout, Sequence: sequence}}, outputs); e != nil {
		return t, e
	}
	var signed struct {
		Hex      string `json:"hex"`
		Complete bool   `json:"complete"`
	}
	if e := n.Wallet.Call(ctx, "signrawtransactionwithwallet", &signed, raw); e != nil {
		return t, e
	}
	if !signed.Complete {
		return t, fmt.Errorf("incomplete transaction signature")
	}
	if e := n.RPC.Call(ctx, "decoderawtransaction", &t, signed.Hex); e != nil {
		return t, e
	}
	t.Hex = signed.Hex
	t.Fee = fee
	return t, nil
}
func (n *Node) Accept(ctx context.Context, t model.Tx) (Acceptance, error) {
	var r []Acceptance
	e := n.RPC.Call(ctx, "testmempoolaccept", &r, []string{t.Hex})
	if e != nil {
		return Acceptance{}, e
	}
	if len(r) != 1 || r[0].TxID != t.TxID {
		return Acceptance{}, fmt.Errorf("malformed acceptance response")
	}
	return r[0], nil
}
func (n *Node) Broadcast(ctx context.Context, t model.Tx) error {
	var id string
	if e := n.RPC.Call(ctx, "sendrawtransaction", &id, t.Hex); e != nil {
		return e
	}
	if id != t.TxID {
		return fmt.Errorf("broadcast identity mismatch")
	}
	return nil
}
func (n *Node) Mempool(ctx context.Context) ([]string, error) {
	var ids []string
	e := n.RPC.Call(ctx, "getrawmempool", &ids)
	return ids, e
}
