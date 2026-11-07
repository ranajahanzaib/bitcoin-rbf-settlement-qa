package model

import (
	"encoding/json"
	"testing"
)

func TestSatoshiAmounts(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want Sats
		bad  bool
	}{{"0.00000001", 1, false}, {"0.00050000", 50000, false}, {"50", 5000000000, false}, {"-0.00000001", -1, false}, {"21000000", 2100000000000000, false}, {"21000001", 0, true}, {"0.000000001", 0, true}, {"1e-8", 0, true}, {"null", 0, true}} {
		t.Run(c.raw, func(t *testing.T) {
			var s Sats
			err := json.Unmarshal([]byte(c.raw), &s)
			if c.bad {
				if err == nil {
					t.Fatal("expected invalid amount")
				}
				return
			}
			if err != nil || s != c.want {
				t.Fatalf("%d %v", s, err)
			}
			var back Sats
			if err = json.Unmarshal([]byte(s.BTC()), &back); err != nil || back != s {
				t.Fatalf("roundtrip %s %v", s.BTC(), err)
			}
		})
	}
}

func TestTransactionJSONAmounts(t *testing.T) {
	tx := Tx{TxID: "test", Fee: 2000, Vout: []Output{{N: 0, Value: 50000}}}
	b, err := json.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	var got Tx
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Vout[0].Value != 50000 || got.Fee != 2000 {
		t.Fatalf("amount drift: %s", b)
	}
}
