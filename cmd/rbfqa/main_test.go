package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCLIInitAndConfigurationErrors(t *testing.T) {
	p := filepath.Join(t.TempDir(), "custom.yaml")
	if code := run([]string{"init", "--config", p}); code != 0 {
		t.Fatalf("init exit=%d", code)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"init", "--config", p}); code != 2 {
		t.Fatal("init overwrote config")
	}
	for _, args := range [][]string{{"run", "unknown"}, {"run"}, {"list", "--format", "invalid"}, {"list", "--timeout", "bad"}, {"run", "normal-confirmation", "--bitcoind", "/nonexistent/rbfqa-bitcoind", "--output", t.TempDir()}} {
		if code := run(args); code != 2 {
			t.Fatalf("args=%v exit=%d", args, code)
		}
	}
}
