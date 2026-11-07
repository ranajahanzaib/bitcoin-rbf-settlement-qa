package observer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/core"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/model"
)

// Capture reads a consistent active chain and mempool, retrying if the tip changes.
// Full regtest scans intentionally trade throughput for restart correctness.
func Capture(ctx context.Context, r *core.RPC, revision int64, previous model.Event) (model.Event, error) {
	for attempt := 0; attempt < 5; attempt++ {
		e := model.Event{Revision: revision, Blocks: []string{}, Transactions: []model.Tx{}}
		var info struct {
			Chain  string `json:"chain"`
			Blocks int    `json:"blocks"`
			Best   string `json:"bestblockhash"`
		}
		if err := r.Call(ctx, "getblockchaininfo", &info); err != nil {
			return e, err
		}
		if info.Chain != "regtest" {
			return e, fmt.Errorf("observer refuses network %s", info.Chain)
		}
		e.Height = info.Blocks
		e.Tip = info.Best
		for h := 0; h <= info.Blocks; h++ {
			var hash string
			if err := r.Call(ctx, "getblockhash", &hash, h); err != nil {
				return e, err
			}
			e.Blocks = append(e.Blocks, hash)
			var b model.Block
			if err := r.Call(ctx, "getblock", &b, hash, 2); err != nil {
				return e, err
			}
			for _, t := range b.Tx {
				if len(t.Vin) > 0 && t.Vin[0].TxID != "" {
					t.Height = h
					t.BlockHash = hash
					t.Confirmations = info.Blocks - h + 1
					t.Status = "confirmed"
					e.Transactions = append(e.Transactions, t)
				}
			}
		}
		var pool []string
		if err := r.Call(ctx, "getrawmempool", &pool); err != nil {
			return e, err
		}
		for _, id := range pool {
			var t model.Tx
			if err := r.Call(ctx, "getrawtransaction", &t, id, true); err != nil {
				return e, err
			}
			t.Status = "mempool"
			t.Height = -1
			t.BlockHash = ""
			t.Confirmations = 0
			e.Transactions = append(e.Transactions, t)
		}
		var end string
		if err := r.Call(ctx, "getbestblockhash", &end); err != nil {
			return e, err
		}
		if end != e.Tip {
			continue
		}
		for h, hash := range previous.Blocks {
			if h >= len(e.Blocks) || e.Blocks[h] != hash {
				e.Disconnected = append(e.Disconnected, previous.Blocks[h:]...)
				break
			}
		}
		data, _ := json.Marshal(e)
		sum := sha256.Sum256(data)
		e.ID = hex.EncodeToString(sum[:])
		return e, nil
	}
	return model.Event{}, fmt.Errorf("active tip changed repeatedly during observation")
}
