package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRPCStructuredErrorAndHTTPFailure(t *testing.T) {
	for _, c := range []struct {
		name, body string
		status     int
		code       int
	}{{"policy", "{\"result\":null,\"error\":{\"code\":-26,\"message\":\"policy rejected\"}}", 500, -26}, {"infra", "unavailable", 503, 0}, {"success", "{\"result\":{\"chain\":\"regtest\"},\"error\":null}", 200, 0}} {
		t.Run(c.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				u, p, ok := r.BasicAuth()
				if !ok || u != "qa" || p != "secret" {
					t.Error("missing auth")
				}
				w.WriteHeader(c.status)
				w.Write([]byte(c.body))
			}))
			defer s.Close()
			r := RPC{URL: s.URL, User: "qa", Password: "secret"}
			var out map[string]any
			err := r.Call(context.Background(), "test", &out)
			if c.name == "success" {
				if err != nil || out["chain"] != "regtest" {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if c.code != 0 {
				var rpc *RPCError
				if !errors.As(err, &rpc) || rpc.Code != c.code {
					t.Fatal("lost structured rejection", err)
				}
			}
		})
	}
}

func TestFundingSelectionHasFeeHeadroom(t *testing.T) {
	u, err := selectFunding([]UTXO{{TxID: "a-merchant-payment", Amount: 50000}, {TxID: "z-funding", Amount: 5000000000}, {TxID: "b-funding", Amount: 4999930000}})
	if err != nil || u.TxID != "b-funding" {
		t.Fatalf("unsafe or nondeterministic funding: %+v %v", u, err)
	}
	if _, err = selectFunding([]UTXO{{TxID: "tiny", Amount: 30000}}); err == nil {
		t.Fatal("insufficient funding accepted")
	}
}
