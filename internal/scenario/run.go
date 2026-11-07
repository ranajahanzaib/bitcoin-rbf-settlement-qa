package scenario

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/adapter"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/config"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/core"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/invariant"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/model"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/observer"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/report"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/settlement"
)

type Metadata struct{ ID, Description string }

var Catalog = []Metadata{{"normal-confirmation", "Normal payment at configured confirmation threshold"}, {"rbf-preserve-payment", "Higher-fee conflicting spend preserves merchant amount"}, {"rbf-payment-redirect", "Replacement removes merchant output"}, {"rbf-amount-reduction", "Replacement reduces merchant payment to 30,000 sats"}, {"rbf-insufficient-fee", "Core rejects a non-increasing replacement fee"}, {"rbf-replacement-chain", "A → B → C replacement across observer restart"}, {"rbf-nonsignaling-original", "Full-RBF replaces an original with final input sequence"}, {"reorg-competing-spend", "Two-node competing-chain reorg, preserved and redirected variants"}, {"duplicate-observer-events", "Duplicate, delayed and restarted observation delivery"}}

func Exists(id string) bool {
	for _, s := range Catalog {
		if s.ID == id {
			return true
		}
	}
	return false
}

type runner struct {
	ctx      context.Context
	cfg      config.Config
	r        *report.Report
	n        *core.Node
	m        adapter.Merchant
	engine   *settlement.Engine
	db       string
	invoice  model.Invoice
	previous model.Event
	revision int64
}

func Run(ctx context.Context, c config.Config, id string) (r report.Report, err error) {
	r = report.Report{Scenario: id, Adapter: c.Adapter, ScenarioVersion: 1, RunID: time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + id, Started: time.Now().UTC(), Network: "regtest"}
	r.ArtifactDir = filepath.Join(c.Output, r.RunID)
	if !Exists(id) {
		r.ExitCode = 2
		return r, fmt.Errorf("unknown scenario %q", id)
	}
	if e := os.MkdirAll(r.ArtifactDir, 0700); e != nil {
		return r, e
	}
	x := &runner{ctx: ctx, cfg: c, r: &r}
	defer func() {
		if x.engine != nil {
			_ = x.engine.Close()
		}
		if x.n != nil {
			x.copyLogs(x.n, "node-a")
			x.n.Close(c.Keep)
		}
		if err != nil {
			r.Failure = err.Error()
			if errors.Is(err, core.ErrEnvironment) {
				r.ExitCode = 2
			} else if r.ExitCode == 0 {
				r.ExitCode = 3
			}
		}
		if e := report.Write(&r); e != nil && err == nil {
			err = e
			r.ExitCode = 3
		}
	}()
	if err = x.setup(); err != nil {
		return
	}
	if strings.HasPrefix(id, "reorg-") {
		err = x.reorg(false)
		if err == nil {
			err = x.newInvoice("redirect")
			if err == nil {
				err = x.reorg(true)
			}
		}
	} else {
		err = x.payment(id)
	}
	if err == nil && !r.Passed() {
		r.ExitCode = 1
	}
	return
}
func (x *runner) copyLogs(n *core.Node, label string) {
	for _, f := range []string{"process.log", "regtest/debug.log"} {
		if b, e := os.ReadFile(filepath.Join(n.Dir, f)); e == nil {
			_ = os.WriteFile(filepath.Join(x.r.ArtifactDir, label+"-"+filepath.Base(f)), b, 0600)
		}
	}
}
func (x *runner) setup() error {
	if err := x.cfg.Validate(); err != nil {
		return fmt.Errorf("%w: %v", core.ErrEnvironment, err)
	}
	if x.cfg.Adapter != "reference" {
		if _, err := adapter.NewHTTP(x.cfg.Adapter, ""); err != nil {
			return fmt.Errorf("%w: %v", core.ErrEnvironment, err)
		}
	}
	var e error
	x.n, e = core.Start(x.ctx, x.cfg.Bitcoind)
	if e != nil {
		return e
	}
	x.r.CoreVersion = x.n.Version
	x.r.Policy = x.n.Policy
	x.r.Nodes = append(x.r.Nodes, "node-a (managed loopback)")
	if e = x.n.Mine(x.ctx, 101); e != nil {
		return e
	}
	x.db = filepath.Join(x.r.ArtifactDir, "merchant.sqlite")
	if x.cfg.Adapter == "reference" {
		x.engine, e = settlement.Open(x.db)
		if e != nil {
			return e
		}
		x.m = x.engine
	} else {
		x.m, e = adapter.NewHTTP(x.cfg.Adapter, os.Getenv(x.cfg.TokenEnv))
		if e != nil {
			return e
		}
	}
	return x.newInvoice("payment")
}
func (x *runner) newInvoice(suffix string) error {
	a, e := x.n.Address(x.ctx)
	if e != nil {
		return e
	}
	required := x.cfg.Confirmations
	if x.r.Scenario == "reorg-competing-spend" {
		required = 1
	}
	x.invoice = model.Invoice{ID: x.r.RunID + "-" + suffix, Address: a, Expected: 50000, Required: required}
	return x.m.Create(x.ctx, x.invoice)
}
func (x *runner) restart() error {
	if x.engine == nil {
		return nil
	}
	if e := x.engine.Close(); e != nil {
		return e
	}
	e, err := settlement.Open(x.db)
	if err != nil {
		return err
	}
	x.engine = e
	x.m = e
	p, err := e.Previous(x.ctx)
	if err != nil {
		return err
	}
	x.previous = p
	x.revision = p.Revision
	return nil
}
func (x *runner) check(name string, capture bool) error {
	if capture {
		x.revision++
		v, e := observer.Capture(x.ctx, x.n.RPC, x.revision, x.previous)
		if e != nil {
			return e
		}
		x.previous = v
		x.r.Events = append(x.r.Events, v)
		if e = x.m.Apply(x.ctx, v); e != nil {
			return e
		}
	}
	s, e := x.m.State(x.ctx, x.invoice.ID)
	if e != nil {
		return e
	}
	x.assert(name+"-invoice-identity", s.Invoice.ID == x.invoice.ID && s.Invoice.Address == x.invoice.Address && s.Invoice.Expected == x.invoice.Expected && s.Invoice.Required == x.invoice.Required, "Merchant preserves invoice identity, amount, address and confirmation policy")
	o, e := invariant.Oracle(x.ctx, x.n.RPC, x.invoice)
	if e != nil {
		return e
	}
	x.r.Snapshots = append(x.r.Snapshots, report.Snapshot{Name: name, State: s, Oracle: o})
	x.r.Assertions = append(x.r.Assertions, invariant.Check(s, o, name)...)
	return nil
}
func (x *runner) assert(name string, ok bool, message string) {
	x.r.Assertions = append(x.r.Assertions, invariant.Result{ID: "SCENARIO", Passed: ok, Message: message, Checkpoint: name})
	if !ok {
		x.r.ExitCode = 1
	}
}
func (x *runner) broadcast(n *core.Node, t model.Tx, expected bool) error {
	t.MerchantAddress = x.invoice.Address
	for _, o := range t.Vout {
		if o.Script.Address == x.invoice.Address {
			t.MerchantAmount += int64(o.Value)
		}
	}
	if t.Vsize > 0 {
		t.FeeRate = fmt.Sprintf("%d/%d sat/vB", t.Fee, t.Vsize)
	}
	a, e := n.Accept(x.ctx, t)
	if e != nil {
		return e
	}
	x.r.Acceptance = append(x.r.Acceptance, a)
	x.r.Transactions = append(x.r.Transactions, t)
	if a.Allowed != expected {
		message := fmt.Sprintf("transaction %s acceptance=%v expected=%v reason=%s %s", t.TxID, a.Allowed, expected, a.RejectReason, a.Details)
		x.assert("mempool-acceptance", false, message)
		return fmt.Errorf("%s", message)
	}
	if expected {
		return n.Broadcast(x.ctx, t)
	}
	if a.RejectReason == "" && a.Details == "" {
		return fmt.Errorf("Core returned rejection without evidence")
	}
	return nil
}
func (x *runner) funding() (core.UTXO, string, string, error) {
	u, e := x.n.Funding(x.ctx)
	if e != nil {
		return u, "", "", e
	}
	change, e := x.n.Address(x.ctx)
	if e != nil {
		return u, "", "", e
	}
	other, e := x.n.Address(x.ctx)
	return u, change, other, e
}
func (x *runner) payment(id string) error {
	u, change, other, e := x.funding()
	if e != nil {
		return e
	}
	seq := uint32(0xfffffffd)
	if id == "rbf-nonsignaling-original" {
		seq = 0xffffffff
	}
	a, e := x.n.Build(x.ctx, u, x.invoice.Address, change, 50000, 2000, seq)
	if e != nil {
		return e
	}
	if e = x.broadcast(x.n, a, true); e != nil {
		return e
	}
	if e = x.check("original-mempool", true); e != nil {
		return e
	}
	canonical := a
	if strings.HasPrefix(id, "rbf-") {
		dest := x.invoice.Address
		amount := int64(50000)
		fee := int64(20000)
		if id == "rbf-payment-redirect" {
			dest = other
		}
		if id == "rbf-amount-reduction" {
			amount = 30000
		}
		if id == "rbf-insufficient-fee" {
			fee = 2000
			amount = 49000
		}
		b, er := x.n.Build(x.ctx, u, dest, change, amount, fee, 0xfffffffd)
		if er != nil {
			return er
		}
		b.Previous = a.TxID
		expected := id != "rbf-insufficient-fee"
		if e = x.broadcast(x.n, b, expected); e != nil {
			return e
		}
		pool, er := x.n.Mempool(x.ctx)
		if er != nil {
			return er
		}
		hasA, hasB := false, false
		for _, t := range pool {
			hasA = hasA || t == a.TxID
			hasB = hasB || t == b.TxID
		}
		x.assert("replacement-policy", hasB == expected && hasA != expected, "Core mempool contains precisely the accepted conflicting version")
		if expected {
			canonical = b
		}
		if e = x.check("replacement-observed", true); e != nil {
			return e
		}
		if id == "rbf-replacement-chain" {
			if e = x.restart(); e != nil {
				return e
			}
			c, er := x.n.Build(x.ctx, u, x.invoice.Address, change, 50000, 40000, 0xfffffffd)
			if er != nil {
				return er
			}
			c.Previous = b.TxID
			if e = x.broadcast(x.n, c, true); e != nil {
				return e
			}
			canonical = c
			if e = x.check("third-version-after-restart", true); e != nil {
				return e
			}
		}
	}
	if e = x.n.Mine(x.ctx, 1); e != nil {
		return e
	}
	if e = x.check("first-confirmation", true); e != nil {
		return e
	}
	if x.invoice.Required > 1 {
		if e = x.n.Mine(x.ctx, x.invoice.Required-1); e != nil {
			return e
		}
	}
	if e = x.check("settled", true); e != nil {
		return e
	}
	var raw model.Tx
	if e = x.n.RPC.Call(x.ctx, "getrawtransaction", &raw, canonical.TxID, true); e != nil {
		return e
	}
	x.assert("canonical-confirmation", raw.Confirmations >= x.invoice.Required, "Accepted transaction has required active-chain confirmations")
	if e = x.check("repeat-reconciliation", true); e != nil {
		return e
	}
	if id == "duplicate-observer-events" {
		if e = x.m.Apply(x.ctx, x.previous); e != nil {
			return e
		}
		if e = x.restart(); e != nil {
			return e
		}
		for _, v := range x.r.Events {
			if e = x.m.Apply(x.ctx, v); e != nil {
				return e
			}
		}
		if e = x.check("duplicate-stale-restart-replay", false); e != nil {
			return e
		}
	}
	s, e := x.m.State(x.ctx, x.invoice.ID)
	if e != nil {
		return e
	}
	want := int64(50000)
	if id == "rbf-payment-redirect" {
		want = 0
	}
	if id == "rbf-amount-reduction" {
		want = 30000
	}
	x.assert("final-amount", s.Balance == want, fmt.Sprintf("Expected canonical recognized amount %d; got %d", want, s.Balance))
	x.assert("one-credit", len(s.Entries) == boolInt(want > 0), "One journal credit per eligible payment output")
	return nil
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func (x *runner) reorg(redirect bool) error {
	b, e := core.Start(x.ctx, x.cfg.Bitcoind)
	if e != nil {
		return e
	}
	defer func() { x.copyLogs(b, "node-b"); b.Close(x.cfg.Keep) }()
	x.r.Nodes = append(x.r.Nodes, "node-b (managed isolated peer)")
	if e = x.n.Connect(x.ctx, b); e != nil {
		return e
	}
	syncCtx, cancel := context.WithTimeout(x.ctx, 20*time.Second)
	e = core.Sync(syncCtx, x.n, b)
	cancel()
	if e != nil {
		return e
	}
	u, change, other, e := x.funding()
	if e != nil {
		return e
	}
	a, e := x.n.Build(x.ctx, u, x.invoice.Address, change, 50000, 2000, 0xfffffffd)
	if e != nil {
		return e
	}
	dest := x.invoice.Address
	if redirect {
		dest = other
	}
	competing, e := x.n.Build(x.ctx, u, dest, change, 50000, 20000, 0xfffffffd)
	if e != nil {
		return e
	}
	competing.Previous = a.TxID
	// Disable all networking on both nodes; wait for peer disappearance before branch creation.
	for _, n := range []*core.Node{x.n, b} {
		if e = n.RPC.Call(x.ctx, "setnetworkactive", nil, false); e != nil {
			return e
		}
		if e = core.Wait(x.ctx, func() (bool, error) {
			var peers []any
			e := n.RPC.Call(x.ctx, "getpeerinfo", &peers)
			return len(peers) == 0, e
		}); e != nil {
			return e
		}
	}
	if e = x.broadcast(x.n, a, true); e != nil {
		return e
	}
	if e = x.broadcast(b, competing, true); e != nil {
		return e
	}
	if e = x.n.Mine(x.ctx, 1); e != nil {
		return e
	}
	if e = b.Mine(x.ctx, 1); e != nil {
		return e
	}
	if e = x.check("losing-branch-settled", true); e != nil {
		return e
	}
	s, e := x.m.State(x.ctx, x.invoice.ID)
	if e != nil {
		return e
	}
	x.assert("pre-reorg-credit", s.Balance == 50000, "Losing branch credited merchant before reorganization")
	var oldHash string
	if e = x.n.RPC.Call(x.ctx, "getbestblockhash", &oldHash); e != nil {
		return e
	}
	if e = b.Mine(x.ctx, 2); e != nil {
		return e
	}
	for _, n := range []*core.Node{x.n, b} {
		if e = n.RPC.Call(x.ctx, "setnetworkactive", nil, true); e != nil {
			return e
		}
	}
	if e = x.n.Connect(x.ctx, b); e != nil {
		return e
	}
	syncCtx, cancel = context.WithTimeout(x.ctx, 30*time.Second)
	e = core.Sync(syncCtx, x.n, b)
	cancel()
	if e != nil {
		return e
	}
	var header struct {
		Confirmations int `json:"confirmations"`
	}
	if e = x.n.RPC.Call(x.ctx, "getblockheader", &header, oldHash); e != nil {
		return e
	}
	x.assert("actual-disconnection", header.Confirmations == -1, "Original credited block no longer belongs to active chain")
	if e = x.check("winning-branch-reconciled", true); e != nil {
		return e
	}
	x.assert("disconnected-block-evidence", len(x.previous.Disconnected) > 0, "Observer identifies common ancestor and disconnected blocks")
	if e = x.restart(); e != nil {
		return e
	}
	if e = x.check("post-reorg-restart", true); e != nil {
		return e
	}
	s, e = x.m.State(x.ctx, x.invoice.ID)
	if e != nil {
		return e
	}
	want := int64(50000)
	if redirect {
		want = 0
	}
	x.assert("reorg-net", s.Balance == want && s.Reversed == 50000, "Orphan credit reversed and canonical merchant amount recognized")
	return nil
}
