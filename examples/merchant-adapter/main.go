// A loopback-only opt-in QA bridge. Do not expose these endpoints in production.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/model"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/settlement"
)

func main() {
	db := flag.String("db", "", "required SQLite file outside your source tree")
	listen := flag.String("listen", "127.0.0.1:8787", "loopback bind address")
	flag.Parse()
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("listen must use a loopback IP")
	}
	if *db == "" {
		log.Fatal("--db is required")
	}
	e, err := settlement.Open(*db)
	if err != nil {
		log.Fatal(err)
	}
	defer e.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/qa/", func(w http.ResponseWriter, r *http.Request) {
		token := os.Getenv("RBFQA_ADAPTER_TOKEN")
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", 401)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
		var err error
		var result any
		switch {
		case r.Method == "POST" && r.URL.Path == "/qa/invoices":
			var i model.Invoice
			err = json.NewDecoder(r.Body).Decode(&i)
			if err == nil {
				err = e.Create(r.Context(), i)
			}
		case r.Method == "POST" && r.URL.Path == "/qa/events":
			var v model.Event
			err = json.NewDecoder(r.Body).Decode(&v)
			if err == nil {
				err = e.Apply(r.Context(), v)
			}
		case r.Method == "POST" && r.URL.Path == "/qa/reconcile": // Events are reconciled atomically on delivery.
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/qa/invoices/"):
			id := strings.TrimPrefix(r.URL.Path, "/qa/invoices/")
			ledger := strings.HasSuffix(id, "/ledger")
			id = strings.TrimSuffix(id, "/ledger")
			var s model.State
			s, err = e.State(r.Context(), id)
			if ledger {
				result = s.Entries
			} else {
				result = s
			}
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if result != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(result)
		} else {
			w.WriteHeader(204)
		}
	})
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	go func() {
		<-ctx.Done()
		shutdown, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("test-only QA adapter listening at %s", *listen)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
