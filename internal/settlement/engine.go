package settlement

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/model"
	_ "modernc.org/sqlite"
)

type Engine struct{ DB *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS schema_version(version INTEGER PRIMARY KEY);
INSERT OR IGNORE INTO schema_version VALUES(1);
CREATE TABLE IF NOT EXISTS invoices(id TEXT PRIMARY KEY,address TEXT UNIQUE NOT NULL,expected INTEGER NOT NULL CHECK(expected>0),required INTEGER NOT NULL CHECK(required>0),status TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS observed_transactions(txid TEXT PRIMARY KEY,metadata BLOB NOT NULL,status TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS transaction_inputs(txid TEXT NOT NULL,previous_txid TEXT NOT NULL,vout INTEGER NOT NULL,PRIMARY KEY(txid,previous_txid,vout));
CREATE INDEX IF NOT EXISTS input_outpoint ON transaction_inputs(previous_txid,vout);
CREATE TABLE IF NOT EXISTS payment_outputs(output TEXT PRIMARY KEY,txid TEXT NOT NULL,vout INTEGER NOT NULL,invoice TEXT NOT NULL REFERENCES invoices(id),amount INTEGER NOT NULL,eligible INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS transaction_conflicts(a TEXT NOT NULL,b TEXT NOT NULL,outpoint TEXT NOT NULL,evidence TEXT NOT NULL,PRIMARY KEY(a,b,outpoint));
CREATE TABLE IF NOT EXISTS chain_checkpoints(height INTEGER PRIMARY KEY,hash TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS settlement_entries(id INTEGER PRIMARY KEY AUTOINCREMENT,key TEXT UNIQUE NOT NULL,invoice TEXT NOT NULL REFERENCES invoices(id),output TEXT NOT NULL,delta INTEGER NOT NULL,reversal INTEGER UNIQUE REFERENCES settlement_entries(id));
CREATE TABLE IF NOT EXISTS processed_events(id TEXT PRIMARY KEY,revision INTEGER NOT NULL,tip TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS observer_state(singleton INTEGER PRIMARY KEY CHECK(singleton=1),revision INTEGER NOT NULL,tip TEXT NOT NULL,event BLOB NOT NULL);
CREATE TRIGGER IF NOT EXISTS journal_no_update BEFORE UPDATE ON settlement_entries BEGIN SELECT RAISE(ABORT,'journal is append-only'); END;
CREATE TRIGGER IF NOT EXISTS journal_no_delete BEFORE DELETE ON settlement_entries BEGIN SELECT RAISE(ABORT,'journal is append-only'); END;
`

func Open(path string) (*Engine, error) {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{"PRAGMA foreign_keys=ON", "PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA busy_timeout=5000", schema} {
		if _, e = db.Exec(q); e != nil {
			db.Close()
			return nil, e
		}
	}
	return &Engine{db}, nil
}
func (e *Engine) Close() error { return e.DB.Close() }
func (e *Engine) Create(ctx context.Context, i model.Invoice) error {
	if i.ID == "" || i.Address == "" || i.Expected <= 0 || i.Required < 1 {
		return fmt.Errorf("invalid invoice")
	}
	_, err := e.DB.ExecContext(ctx, "INSERT INTO invoices VALUES(?,?,?,?,?)", i.ID, i.Address, i.Expected, i.Required, "unpaid")
	return err
}
func (e *Engine) Previous(ctx context.Context) (model.Event, error) {
	var b []byte
	err := e.DB.QueryRowContext(ctx, "SELECT event FROM observer_state WHERE singleton=1").Scan(&b)
	if err == sql.ErrNoRows {
		return model.Event{}, nil
	}
	var v model.Event
	if err == nil {
		err = json.Unmarshal(b, &v)
	}
	return v, err
}
func (e *Engine) Apply(ctx context.Context, v model.Event) (err error) {
	if v.ID == "" || v.Revision <= 0 || v.Height < 0 || len(v.Blocks) != v.Height+1 || v.Blocks[v.Height] != v.Tip {
		return fmt.Errorf("observation requires authoritative chain context; reconciliation required")
	}
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var rev int64
	var tip string
	err = tx.QueryRowContext(ctx, "SELECT revision,tip FROM observer_state WHERE singleton=1").Scan(&rev, &tip)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if v.Revision < rev {
		return nil
	}
	if v.Revision == rev {
		var eventID string
		if er := tx.QueryRowContext(ctx, "SELECT id FROM processed_events WHERE revision=? ORDER BY rowid DESC LIMIT 1", rev).Scan(&eventID); er != nil {
			return er
		}
		if tip != v.Tip || eventID != v.ID {
			return fmt.Errorf("ambiguous revision: reconciliation required")
		}
		return nil
	}
	var seen int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM processed_events WHERE id=?", v.ID).Scan(&seen); err != nil {
		return err
	}
	if seen > 0 {
		return nil
	}
	exec := func(q string, args ...any) error { _, e := tx.ExecContext(ctx, q, args...); return e }
	var invoices []model.Invoice
	rows, err := tx.QueryContext(ctx, "SELECT id,address,expected,required,status FROM invoices")
	if err != nil {
		return err
	}
	for rows.Next() {
		var i model.Invoice
		if err = rows.Scan(&i.ID, &i.Address, &i.Expected, &i.Required, &i.Status); err != nil {
			rows.Close()
			return err
		}
		invoices = append(invoices, i)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	watched := map[string]model.Invoice{}
	for _, i := range invoices {
		watched[i.Address] = i
	}
	known := map[string]bool{}
	rows, err = tx.QueryContext(ctx, "SELECT previous_txid,vout FROM transaction_inputs")
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		var n int
		if err = rows.Scan(&id, &n); err != nil {
			rows.Close()
			return err
		}
		known[fmt.Sprintf("%s:%d", id, n)] = true
	}
	rows.Close()
	for _, t := range v.Transactions {
		for _, o := range t.Vout {
			if _, ok := watched[o.Script.Address]; ok {
				for _, in := range t.Vin {
					known[fmt.Sprintf("%s:%d", in.TxID, in.Vout)] = true
				}
			}
		}
	}
	if err = exec("UPDATE observed_transactions SET status='unknown'"); err != nil {
		return err
	}
	if err = exec("UPDATE payment_outputs SET eligible=0"); err != nil {
		return err
	}
	for _, t := range v.Transactions {
		relevant := false
		for _, in := range t.Vin {
			if known[fmt.Sprintf("%s:%d", in.TxID, in.Vout)] {
				relevant = true
			}
		}
		for _, o := range t.Vout {
			if _, ok := watched[o.Script.Address]; ok {
				relevant = true
			}
		}
		if !relevant {
			continue
		}
		if t.Status == "confirmed" && (t.Height < 0 || t.Height >= len(v.Blocks) || v.Blocks[t.Height] != t.BlockHash || t.Confirmations != v.Height-t.Height+1) {
			return fmt.Errorf("inconsistent transaction chain context")
		}
		b, er := json.Marshal(t)
		if er != nil {
			return er
		}
		if err = exec("INSERT INTO observed_transactions VALUES(?,?,?) ON CONFLICT(txid) DO UPDATE SET metadata=excluded.metadata,status=excluded.status", t.TxID, b, t.Status); err != nil {
			return err
		}
		for _, in := range t.Vin {
			if err = exec("INSERT OR IGNORE INTO transaction_inputs VALUES(?,?,?)", t.TxID, in.TxID, in.Vout); err != nil {
				return err
			}
		}
		for _, o := range t.Vout {
			if i, ok := watched[o.Script.Address]; ok {
				eligible := t.Status == "confirmed" && t.Confirmations >= i.Required
				if err = exec("INSERT INTO payment_outputs VALUES(?,?,?,?,?,?) ON CONFLICT(output) DO UPDATE SET eligible=excluded.eligible", fmt.Sprintf("%s:%d", t.TxID, o.N), t.TxID, o.N, i.ID, int64(o.Value), eligible); err != nil {
					return err
				}
			}
		}
	}
	if err = exec(`INSERT OR IGNORE INTO transaction_conflicts SELECT a.txid,b.txid,a.previous_txid||':'||a.vout,'shared spent outpoint' FROM transaction_inputs a JOIN transaction_inputs b ON a.previous_txid=b.previous_txid AND a.vout=b.vout AND a.txid<b.txid`); err != nil {
		return err
	}
	if err = exec(`UPDATE observed_transactions SET status='conflicted' WHERE status='unknown' AND txid IN (SELECT a FROM transaction_conflicts WHERE b IN (SELECT txid FROM observed_transactions WHERE status IN ('mempool','confirmed')) UNION SELECT b FROM transaction_conflicts WHERE a IN (SELECT txid FROM observed_transactions WHERE status IN ('mempool','confirmed')))`); err != nil {
		return err
	}
	// Reverse every active credit whose output lost canonical eligibility.
	rows, err = tx.QueryContext(ctx, `SELECT e.id,e.invoice,e.output,e.delta FROM settlement_entries e JOIN payment_outputs p ON p.output=e.output WHERE e.delta>0 AND p.eligible=0 AND NOT EXISTS(SELECT 1 FROM settlement_entries r WHERE r.reversal=e.id)`)
	if err != nil {
		return err
	}
	var reverse []model.Entry
	for rows.Next() {
		var x model.Entry
		if err = rows.Scan(&x.ID, &x.Invoice, &x.Output, &x.Delta); err != nil {
			rows.Close()
			return err
		}
		reverse = append(reverse, x)
	}
	rows.Close()
	for _, x := range reverse {
		if err = exec("INSERT INTO settlement_entries(key,invoice,output,delta,reversal) VALUES(?,?,?,?,?)", fmt.Sprintf("reverse:%d", x.ID), x.Invoice, x.Output, -x.Delta, x.ID); err != nil {
			return err
		}
	}
	rows, err = tx.QueryContext(ctx, `SELECT p.output,p.invoice,p.amount FROM payment_outputs p WHERE p.eligible=1 AND NOT EXISTS(SELECT 1 FROM settlement_entries e WHERE e.output=p.output AND e.delta>0 AND NOT EXISTS(SELECT 1 FROM settlement_entries r WHERE r.reversal=e.id))`)
	if err != nil {
		return err
	}
	var credit []model.Entry
	for rows.Next() {
		var x model.Entry
		if err = rows.Scan(&x.Output, &x.Invoice, &x.Delta); err != nil {
			rows.Close()
			return err
		}
		credit = append(credit, x)
	}
	rows.Close()
	for _, x := range credit {
		if err = exec("INSERT INTO settlement_entries(key,invoice,output,delta) VALUES(?,?,?,?)", fmt.Sprintf("credit:%s:%d", x.Output, v.Revision), x.Invoice, x.Output, x.Delta); err != nil {
			return err
		}
	}
	for _, i := range invoices {
		var bal, pending int64
		if err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(delta),0) FROM settlement_entries WHERE invoice=?", i.ID).Scan(&bal); err != nil {
			return err
		}
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(p.amount),0) FROM payment_outputs p JOIN observed_transactions t ON t.txid=p.txid WHERE p.invoice=? AND p.eligible=0 AND t.status IN ('mempool','confirmed')`, i.ID).Scan(&pending); err != nil {
			return err
		}
		status := "unpaid"
		if pending > 0 {
			status = "observed"
		}
		if bal > 0 {
			status = "partially_paid"
		}
		if bal >= i.Expected {
			status = "settled"
		}
		if err = exec("UPDATE invoices SET status=? WHERE id=?", status, i.ID); err != nil {
			return err
		}
	}
	if err = exec("DELETE FROM chain_checkpoints"); err != nil {
		return err
	}
	for h, hash := range v.Blocks {
		if err = exec("INSERT INTO chain_checkpoints VALUES(?,?)", h, hash); err != nil {
			return err
		}
	}
	if err = exec("INSERT INTO processed_events VALUES(?,?,?)", v.ID, v.Revision, v.Tip); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err = exec("INSERT INTO observer_state VALUES(1,?,?,?) ON CONFLICT(singleton) DO UPDATE SET revision=excluded.revision,tip=excluded.tip,event=excluded.event", v.Revision, v.Tip, data); err != nil {
		return err
	}
	return tx.Commit()
}
func (e *Engine) State(ctx context.Context, id string) (s model.State, err error) {
	err = e.DB.QueryRowContext(ctx, "SELECT id,address,expected,required,status FROM invoices WHERE id=?", id).Scan(&s.Invoice.ID, &s.Invoice.Address, &s.Invoice.Expected, &s.Invoice.Required, &s.Invoice.Status)
	if err != nil {
		return
	}
	rows, err := e.DB.QueryContext(ctx, "SELECT id,key,invoice,output,delta,COALESCE(reversal,0) FROM settlement_entries WHERE invoice=? ORDER BY id", id)
	if err != nil {
		return
	}
	for rows.Next() {
		var x model.Entry
		if err = rows.Scan(&x.ID, &x.Key, &x.Invoice, &x.Output, &x.Delta, &x.Reversal); err != nil {
			rows.Close()
			return
		}
		s.Entries = append(s.Entries, x)
		s.Balance += x.Delta
		if x.Delta > 0 {
			s.Gross += x.Delta
		} else {
			s.Reversed -= x.Delta
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return
	}
	rows, err = e.DB.QueryContext(ctx, "SELECT metadata,status FROM observed_transactions ORDER BY txid")
	if err != nil {
		return
	}
	for rows.Next() {
		var b []byte
		var status string
		if err = rows.Scan(&b, &status); err != nil {
			rows.Close()
			return
		}
		var t model.Tx
		if err = json.Unmarshal(b, &t); err != nil {
			rows.Close()
			return
		}
		t.Status = status
		s.Transactions = append(s.Transactions, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return
	}
	err = e.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(p.amount),0) FROM payment_outputs p JOIN observed_transactions t ON t.txid=p.txid WHERE p.invoice=? AND p.eligible=0 AND t.status IN ('mempool','confirmed')`, id).Scan(&s.Pending)
	if err != nil {
		return
	}
	s.Outstanding = s.Invoice.Expected - s.Balance
	if s.Outstanding < 0 {
		s.Outstanding = 0
	}
	err = e.DB.QueryRowContext(ctx, "SELECT revision,tip FROM observer_state WHERE singleton=1").Scan(&s.Revision, &s.Tip)
	if err == sql.ErrNoRows {
		err = nil
	}
	return
}
