package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	merchantadapter "github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/adapter"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/config"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/core"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/invariant"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/report"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/scenario"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/settlement"
)

func main() { os.Exit(run(os.Args[1:])) }
func run(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		help()
		return 0
	}
	command := args[0]
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	var path, adapter, output, format, timeout, binary string
	var keep, verbose bool
	f.StringVar(&path, "config", "", "YAML configuration")
	f.StringVar(&adapter, "adapter", "", "reference or HTTP(S) origin")
	f.StringVar(&output, "output", "", "artifact directory")
	f.StringVar(&format, "format", "terminal", "terminal, json, markdown")
	f.StringVar(&timeout, "timeout", "", "overall duration (e.g. 3m)")
	f.StringVar(&binary, "bitcoind", "", "Bitcoin Core 32.1 binary")
	f.BoolVar(&keep, "keep-artifacts", false, "preserve managed node datadirs (wallets stay outside repository)")
	f.BoolVar(&verbose, "verbose", false, "print artifact and node diagnostics")
	pos := []string{}
	opts := []string{}
	for j := 1; j < len(args); j++ {
		a := args[j]
		if strings.HasPrefix(a, "-") {
			opts = append(opts, a)
			name := strings.TrimLeft(strings.Split(a, "=")[0], "-")
			if !strings.Contains(a, "=") && name != "keep-artifacts" && name != "verbose" && name != "help" && name != "h" && j+1 < len(args) {
				j++
				opts = append(opts, args[j])
			}
		} else {
			pos = append(pos, a)
		}
	}
	if e := f.Parse(opts); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if format != "terminal" && format != "json" && format != "markdown" {
		return fail(2, fmt.Errorf("format must be terminal, json, or markdown"))
	}
	loadPath := path
	if command == "init" {
		loadPath = ""
	}
	c, e := config.Load(loadPath)
	if e != nil {
		return fail(2, e)
	}
	if adapter != "" {
		c.Adapter = adapter
	}
	if output != "" {
		c.Output = output
	}
	if timeout != "" {
		c.Timeout = timeout
	}
	if binary != "" {
		c.Bitcoind = binary
	}
	if keep {
		c.Keep = true
	}
	if e = c.Validate(); e != nil {
		return fail(2, e)
	}
	duration, _ := time.ParseDuration(c.Timeout)
	base, cancelSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelSignal()
	ctx, cancel := context.WithTimeout(base, duration)
	defer cancel()
	switch command {
	case "init":
		target := "rbfqa.yaml"
		if path != "" {
			target = path
		}
		file, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return fail(2, e)
		}
		_, e = file.WriteString(config.Example)
		closeErr := file.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			return fail(3, e)
		}
		fmt.Println("Created", target)
		return 0
	case "list":
		if format == "json" {
			_ = json.NewEncoder(os.Stdout).Encode(scenario.Catalog)
		} else {
			for _, s := range scenario.Catalog {
				fmt.Printf("%-30s %s\n", s.ID, s.Description)
			}
		}
		return 0
	case "doctor":
		if _, e = exec.LookPath(c.Bitcoind); e != nil {
			return fail(2, fmt.Errorf("install Bitcoin Core 32.1 and set --bitcoind: %w", e))
		}
		if e = os.MkdirAll(c.Output, 0700); e != nil {
			return fail(2, e)
		}
		probe, err := os.CreateTemp(c.Output, ".doctor-*")
		if err != nil {
			return fail(2, err)
		}
		_ = probe.Close()
		_ = os.Remove(probe.Name())
		d, e := os.MkdirTemp("", "rbfqa-doctor-")
		if e != nil {
			return fail(2, e)
		}
		os.Remove(d)
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			return fail(2, e)
		}
		l.Close()
		n, e := core.Start(ctx, c.Bitcoind)
		if e != nil {
			return fail(2, e)
		}
		defer n.Close(false)
		if e = n.Mine(ctx, 1); e != nil {
			return fail(2, e)
		}
		if format == "json" {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": true, "core_version": n.Version, "network": "regtest", "capabilities": true, "wallet": true, "mining": true, "workspace_writable": true, "loopback_ports": true})
			return 0
		}
		fmt.Printf("PASS: Core %d, regtest RPC capabilities, descriptor wallet, block generation, writable storage, loopback ports\n", n.Version)
		if _, e = exec.LookPath("docker"); e == nil {
			fmt.Println("Docker CLI available (optional; daemon not required for native runs)")
		}
		return 0
	case "run":
		if len(pos) != 1 {
			return fail(2, fmt.Errorf("usage: rbfqa run SCENARIO|all [flags]"))
		}
		ids := []string{pos[0]}
		if pos[0] == "all" {
			ids = nil
			for _, s := range scenario.Catalog {
				ids = append(ids, s.ID)
			}
		}
		code := 0
		for _, id := range ids {
			if ctx.Err() != nil {
				return fail(3, ctx.Err())
			}
			if !scenario.Exists(id) {
				return fail(2, fmt.Errorf("unknown scenario %s; use rbfqa list", id))
			}
			r, err := scenario.Run(ctx, c, id)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			render(r, format)
			if verbose {
				fmt.Fprintln(os.Stderr, "Artifacts:", r.ArtifactDir)
			}
			if r.ExitCode > code {
				code = r.ExitCode
			}
			if err != nil && code == 0 {
				code = 3
			}
		}
		return code
	case "report", "replay":
		if len(pos) != 1 {
			return fail(2, fmt.Errorf("usage: rbfqa %s REPORT.json", command))
		}
		b, e := os.ReadFile(pos[0])
		if e != nil {
			return fail(2, e)
		}
		var r report.Report
		if e = json.Unmarshal(b, &r); e != nil {
			return fail(2, e)
		}
		if command == "report" {
			render(r, format)
			return r.ExitCode
		}
		if len(r.Events) == 0 || len(r.Snapshots) == 0 {
			return fail(2, fmt.Errorf("report contains no replayable observations"))
		}
		dir, e := os.MkdirTemp("", "rbfqa-replay-")
		if e != nil {
			return fail(3, e)
		}
		defer os.RemoveAll(dir)
		engine, e := settlement.Open(filepath.Join(dir, "replay.sqlite"))
		if e != nil {
			return fail(3, e)
		}
		defer engine.Close()
		var merchant merchantadapter.Merchant = engine
		if c.Adapter != "reference" {
			h, err := merchantadapter.NewHTTP(c.Adapter, os.Getenv(c.TokenEnv))
			if err != nil {
				return fail(2, err)
			}
			merchant = h
		}
		seen := map[string]bool{}
		for _, s := range r.Snapshots {
			i := s.State.Invoice
			if !seen[i.ID] {
				if e = merchant.Create(ctx, i); e != nil {
					return fail(3, e)
				}
				seen[i.ID] = true
			}
		}
		for _, v := range r.Events {
			if e = merchant.Apply(ctx, v); e != nil {
				return fail(3, e)
			}
		}
		for _, v := range r.Events {
			if e = merchant.Apply(ctx, v); e != nil {
				return fail(3, e)
			}
		}
		for id := range seen {
			s, e := merchant.State(ctx, id)
			if e != nil {
				return fail(3, e)
			}
			var oracle invariant.Evidence
			for _, snap := range r.Snapshots {
				if snap.State.Invoice.ID == id {
					oracle = snap.Oracle
				}
			}
			// Later snapshots may advance the tip after a different invoice's last checkpoint.
			oracle.Tip = r.Events[len(r.Events)-1].Tip
			for _, result := range invariant.Check(s, oracle, "offline-replay") {
				if !result.Passed {
					return fail(1, fmt.Errorf("%s", result.Message))
				}
			}
			switch format {
			case "json":
				_ = json.NewEncoder(os.Stdout).Encode(s)
			case "markdown":
				fmt.Printf("**PASS** offline accounting replay: `%s`, balance %d sats, entries %d\n", id, s.Balance, len(s.Entries))
			default:
				fmt.Printf("PASS offline accounting replay: %s, balance=%d sats, entries=%d\n", id, s.Balance, len(s.Entries))
			}
		}
		return 0
	default:
		return fail(2, fmt.Errorf("unknown command %q", command))
	}
}
func render(r report.Report, format string) {
	switch format {
	case "json":
		_ = json.NewEncoder(os.Stdout).Encode(r)
	case "markdown":
		fmt.Print(r.Markdown())
	default:
		status := "FAIL"
		if r.Passed() {
			status = "PASS"
		}
		passed := 0
		for _, a := range r.Assertions {
			if a.Passed {
				passed++
			} else {
				fmt.Fprintln(os.Stderr, a.Message)
			}
		}
		fmt.Printf("%s %s — Core %d · %d/%d assertions · %s\n", status, r.Scenario, r.CoreVersion, passed, len(r.Assertions), r.ArtifactDir)
	}
}
func fail(code int, e error) int { fmt.Fprintln(os.Stderr, "rbfqa:", e); return code }
func help() {
	fmt.Print(`Bitcoin RBF Settlement QA — real regtest settlement checks

Usage: rbfqa COMMAND [ARGUMENT] [flags]
  doctor                  Verify Core 32.1, RPC and local environment
  init                    Write commented rbfqa.yaml (never overwrite)
  list                    List nine built-in scenarios
  run SCENARIO|all        Execute isolated real regtest scenarios
  replay REPORT.json      Replay captured accounting observations offline
  report REPORT.json      Render a saved report

Flags: --config FILE --bitcoind PATH --adapter reference|URL --output DIR
       --format terminal|json|markdown --timeout 3m --keep-artifacts --verbose

Examples:
  rbfqa doctor --bitcoind /path/to/bitcoind
  rbfqa run rbf-preserve-payment
  rbfqa run all --output artifacts --timeout 10m
  rbfqa replay artifacts/RUN/report.json

Exit codes: 0 pass, 1 settlement/scenario defect, 2 configuration/environment,
3 infrastructure/execution failure. Only managed regtest nodes are supported.
`)
}
