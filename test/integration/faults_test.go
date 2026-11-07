package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/config"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/model"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/scenario"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/settlement"
)

// faultMerchant deliberately creates an extra credit for every confirmed event,
// or exposes orphan credits without their compensating entries. Test-only.
type faultMerchant struct {
	engine   *settlement.Engine
	fault    string
	invoices map[string]bool
	extras   map[string][]model.Entry
}

func (f *faultMerchant) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var out any
	var err error
	switch {
	case r.Method == "POST" && r.URL.Path == "/qa/invoices":
		var i model.Invoice
		err = json.NewDecoder(r.Body).Decode(&i)
		if err == nil {
			err = f.engine.Create(ctx, i)
			f.invoices[i.ID] = true
		}
	case r.Method == "POST" && r.URL.Path == "/qa/events":
		var e model.Event
		err = json.NewDecoder(r.Body).Decode(&e)
		if err == nil {
			err = f.engine.Apply(ctx, e)
		}
		if err == nil && f.fault == "duplicate" {
			for id := range f.invoices {
				s, se := f.engine.State(ctx, id)
				if se != nil {
					err = se
					break
				}
				for _, x := range s.Entries {
					if x.Delta > 0 {
						x.ID = int64(10000 + len(f.extras[id]))
						x.Key = x.Key + "-extra-" + time.Now().String()
						f.extras[id] = append(f.extras[id], x)
					}
				}
			}
		}
	case r.Method == "POST" && r.URL.Path == "/qa/reconcile":
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/qa/invoices/"):
		id := strings.TrimPrefix(r.URL.Path, "/qa/invoices/")
		ledger := strings.HasSuffix(id, "/ledger")
		id = strings.TrimSuffix(id, "/ledger")
		var s model.State
		s, err = f.engine.State(ctx, id)
		if err != nil {
			break
		}
		if f.fault == "duplicate" {
			s.Entries = append(s.Entries, f.extras[id]...)
			for _, x := range f.extras[id] {
				s.Balance += x.Delta
			}
		} else {
			entries := []model.Entry{}
			for _, x := range s.Entries {
				if x.Delta > 0 {
					entries = append(entries, x)
				}
			}
			s.Entries = entries
			s.Balance = 0
			for _, x := range entries {
				s.Balance += x.Delta
			}
		}
		if ledger {
			out = s.Entries
		} else {
			out = s
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if out != nil {
		_ = json.NewEncoder(w).Encode(out)
	} else {
		w.WriteHeader(204)
	}
}
func TestRealCoreDetectsFaultyHTTPMerchants(t *testing.T) {
	binary := os.Getenv("RBFQA_BITCOIND")
	if binary == "" {
		t.Skip("set RBFQA_BITCOIND to test negative detection against real Core")
	}
	for _, c := range []struct{ fault, scenario, invariant string }{{"duplicate", "duplicate-observer-events", "INV-001"}, {"missing-reversal", "reorg-competing-spend", "INV-007"}} {
		t.Run(c.fault, func(t *testing.T) {
			e, err := settlement.Open(filepath.Join(t.TempDir(), "merchant.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			server := httptest.NewServer(&faultMerchant{e, c.fault, map[string]bool{}, map[string][]model.Entry{}})
			defer server.Close()
			cfg, _ := config.Load("")
			cfg.Bitcoind = binary
			cfg.Adapter = server.URL
			cfg.Output = artifactOutput(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			r, err := scenario.Run(ctx, cfg, c.scenario)
			if err != nil {
				t.Fatal(err)
			}
			detected := false
			for _, a := range r.Assertions {
				if a.ID == c.invariant && !a.Passed {
					detected = true
				}
			}
			if !detected || r.ExitCode != 1 {
				t.Fatalf("fault escaped detection, exit=%d assertions=%+v", r.ExitCode, r.Assertions)
			}
		})
	}
}
