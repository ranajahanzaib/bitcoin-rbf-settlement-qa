package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RPC struct {
	URL, User, Password string
	Client              *http.Client
}
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("RPC %d: %s", e.Code, e.Message) }
func (r *RPC) Call(ctx context.Context, method string, out any, args ...any) error {
	if args == nil {
		args = []any{}
	}
	b, e := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "rbfqa", "method": method, "params": args})
	if e != nil {
		return e
	}
	q, e := http.NewRequestWithContext(ctx, "POST", r.URL, bytes.NewReader(b))
	if e != nil {
		return e
	}
	q.SetBasicAuth(r.User, r.Password)
	q.Header.Set("Content-Type", "application/json")
	c := r.Client
	if c == nil {
		c = &http.Client{Timeout: 60 * time.Second}
	}
	resp, e := c.Do(q)
	if e != nil {
		return fmt.Errorf("%s: %w", method, e)
	}
	defer resp.Body.Close()
	var v struct {
		Result json.RawMessage `json:"result"`
		Error  *RPCError       `json:"error"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&v); e != nil {
		return fmt.Errorf("%s HTTP %d: %w", method, resp.StatusCode, e)
	}
	if v.Error != nil {
		return v.Error
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s HTTP %d", method, resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(v.Result, out)
	}
	return nil
}
func Wait(ctx context.Context, f func() (bool, error)) error {
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	var last error
	for {
		ok, e := f()
		if ok && e == nil {
			return nil
		}
		last = e
		select {
		case <-ctx.Done():
			return fmt.Errorf("condition deadline: %w (last: %v)", ctx.Err(), last)
		case <-t.C:
		}
	}
}
