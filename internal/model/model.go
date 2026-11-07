package model

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Sats int64

func (s Sats) MarshalJSON() ([]byte, error) { return []byte(s.BTC()), nil }

func (s *Sats) UnmarshalJSON(b []byte) error {
	v := strings.Trim(string(b), `"`)
	if strings.ContainsAny(v, "eE") {
		return fmt.Errorf("exponent amount unsupported: %s", v)
	}
	p := strings.Split(v, ".")
	if len(p) > 2 {
		return fmt.Errorf("invalid amount")
	}
	n, e := strconv.ParseInt(p[0], 10, 64)
	if e != nil {
		return e
	}
	frac := ""
	if len(p) == 2 {
		frac = p[1]
	}
	if len(frac) > 8 {
		return fmt.Errorf("sub-satoshi amount")
	}
	frac += strings.Repeat("0", 8-len(frac))
	f, e := strconv.ParseInt(frac, 10, 64)
	if e != nil {
		return e
	}
	if strings.HasPrefix(v, "-") {
		f = -f
	}
	if n > 21000000 || n < -21000000 {
		return fmt.Errorf("amount out of range")
	}
	*s = Sats(n*100000000 + f)
	return nil
}
func (s Sats) BTC() json.Number {
	n := int64(s)
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	return json.Number(fmt.Sprintf("%s%d.%08d", sign, n/100000000, n%100000000))
}

type Input struct {
	TxID     string `json:"txid"`
	Vout     int    `json:"vout"`
	Sequence uint32 `json:"sequence,omitempty"`
}
type Output struct {
	N      int  `json:"n"`
	Value  Sats `json:"value"`
	Script struct {
		Address string `json:"address"`
	} `json:"scriptPubKey"`
}
type Tx struct {
	TxID            string   `json:"txid"`
	Hex             string   `json:"hex,omitempty"`
	Vin             []Input  `json:"vin"`
	Vout            []Output `json:"vout"`
	Vsize           int      `json:"vsize"`
	BlockHash       string   `json:"blockhash,omitempty"`
	Height          int      `json:"height"`
	Confirmations   int      `json:"confirmations"`
	Status          string   `json:"status"`
	Fee             int64    `json:"fee_sats"`
	MerchantAddress string   `json:"merchant_address,omitempty"`
	MerchantAmount  int64    `json:"merchant_amount_sats"`
	FeeRate         string   `json:"feerate,omitempty"`
	Previous        string   `json:"previous,omitempty"`
}
type Block struct {
	Hash     string `json:"hash"`
	Height   int    `json:"height"`
	Previous string `json:"previousblockhash"`
	Tx       []Tx   `json:"tx"`
}
type Invoice struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	Expected int64  `json:"expected_sats"`
	Required int    `json:"required_confirmations"`
	Status   string `json:"status"`
}
type Event struct {
	ID           string   `json:"id"`
	Revision     int64    `json:"revision"`
	Tip          string   `json:"tip"`
	Height       int      `json:"height"`
	Blocks       []string `json:"blocks"`
	Transactions []Tx     `json:"transactions"`
	Disconnected []string `json:"disconnected,omitempty"`
}
type Entry struct {
	ID       int64  `json:"id"`
	Key      string `json:"idempotency_key"`
	Invoice  string `json:"invoice_id"`
	Output   string `json:"output"`
	Delta    int64  `json:"delta_sats"`
	Reversal int64  `json:"reversal_of,omitempty"`
}
type State struct {
	Invoice      Invoice `json:"invoice"`
	Balance      int64   `json:"balance_sats"`
	Pending      int64   `json:"pending_sats"`
	Outstanding  int64   `json:"outstanding_sats"`
	Gross        int64   `json:"gross_credits_sats"`
	Reversed     int64   `json:"reversals_sats"`
	Entries      []Entry `json:"entries"`
	Transactions []Tx    `json:"transactions"`
	Revision     int64   `json:"revision"`
	Tip          string  `json:"tip"`
}
