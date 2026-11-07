package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/model"
)

type Merchant interface {
	Create(context.Context, model.Invoice) error
	Apply(context.Context, model.Event) error
	State(context.Context, string) (model.State, error)
}
type HTTP struct {
	Base, Token string
	Client      *http.Client
}

func NewHTTP(base, token string) (*HTTP, error) {
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("adapter must be an HTTP origin without credentials, path, query or fragment")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if u.Scheme != "https" && (u.Scheme != "http" || !(host == "localhost" || ip != nil && ip.IsLoopback())) {
		return nil, fmt.Errorf("adapter requires HTTPS except loopback HTTP")
	}
	return &HTTP{Base: strings.TrimRight(base, "/"), Token: token, Client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (h *HTTP) request(ctx context.Context, method, path string, body, out any) error {
	var b bytes.Buffer
	if body != nil {
		if e := json.NewEncoder(&b).Encode(body); e != nil {
			return e
		}
	}
	q, e := http.NewRequestWithContext(ctx, method, h.Base+path, &b)
	if e != nil {
		return e
	}
	q.Header.Set("Content-Type", "application/json")
	if h.Token != "" {
		q.Header.Set("Authorization", "Bearer "+h.Token)
	}
	resp, e := h.Client.Do(q)
	if e != nil {
		return fmt.Errorf("adapter request failed: %w", e)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("adapter %s %s returned HTTP %d", method, path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out)
	}
	return nil
}
func (h *HTTP) Create(ctx context.Context, i model.Invoice) error {
	return h.request(ctx, "POST", "/qa/invoices", i, nil)
}
func (h *HTTP) Apply(ctx context.Context, e model.Event) error {
	if err := h.request(ctx, "POST", "/qa/events", e, nil); err != nil {
		return err
	}
	return h.request(ctx, "POST", "/qa/reconcile", map[string]any{"revision": e.Revision, "tip": e.Tip}, nil)
}
func (h *HTTP) State(ctx context.Context, id string) (s model.State, err error) {
	err = h.request(ctx, "GET", "/qa/invoices/"+url.PathEscape(id), nil, &s)
	if err != nil {
		return
	}
	err = h.request(ctx, "GET", "/qa/invoices/"+url.PathEscape(id)+"/ledger", nil, &s.Entries)
	return
}
