package config

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Bitcoind      string `yaml:"bitcoind"`
	Output        string `yaml:"output"`
	Adapter       string `yaml:"adapter"`
	TokenEnv      string `yaml:"adapter_token_env"`
	Confirmations int    `yaml:"confirmations"`
	Timeout       string `yaml:"timeout"`
	Keep          bool   `yaml:"keep_artifacts"`
}

const Example = `# Managed, isolated regtest only. No external node or wallet configuration.
bitcoind: bitcoind
output: artifacts
adapter: reference
adapter_token_env: RBFQA_ADAPTER_TOKEN
confirmations: 2
timeout: 3m
keep_artifacts: false
`

func Load(path string) (Config, error) {
	c := Config{Bitcoind: "bitcoind", Output: "artifacts", Adapter: "reference", TokenEnv: "RBFQA_ADAPTER_TOKEN", Confirmations: 2, Timeout: "3m"}
	if path != "" {
		b, e := os.ReadFile(path)
		if e != nil {
			return c, e
		}
		d := yaml.NewDecoder(bytes.NewReader(b))
		d.KnownFields(true)
		if e = d.Decode(&c); e != nil {
			return c, e
		}
	}
	return c, c.Validate()
}
func (c Config) Validate() error {
	if c.Bitcoind == "" || c.Output == "" || c.Confirmations < 1 || c.Confirmations > 100 {
		return fmt.Errorf("bitcoind/output required; confirmations must be 1..100")
	}
	d, e := time.ParseDuration(c.Timeout)
	if e != nil || d <= 0 {
		return fmt.Errorf("timeout must be a positive duration")
	}
	if c.Adapter == "" {
		return fmt.Errorf("adapter required")
	}
	return nil
}
