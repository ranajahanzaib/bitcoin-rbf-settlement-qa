package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	for _, c := range []struct {
		name, body string
		bad        bool
	}{{"defaults", Example, false}, {"mainnet", "network: mainnet", true}, {"invalid threshold", "confirmations: 0", true}, {"invalid timeout", "timeout: forever", true}} {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.yaml")
			os.WriteFile(p, []byte(c.body), 0600)
			_, err := Load(p)
			if (err != nil) != c.bad {
				t.Fatal(err)
			}
		})
	}
}
