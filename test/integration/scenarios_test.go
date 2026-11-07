package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/config"
	"github.com/ranajahanzaib/bitcoin-rbf-settlement-qa/internal/scenario"
)

func TestRealCoreScenarios(t *testing.T) {
	binary := os.Getenv("RBFQA_BITCOIND")
	if binary == "" {
		t.Skip("set RBFQA_BITCOIND to execute real Bitcoin Core 32.1 integration tests")
	}
	for _, s := range scenario.Catalog {
		t.Run(s.ID, func(t *testing.T) {
			c, _ := config.Load("")
			c.Bitcoind = binary
			c.Output = artifactOutput(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			r, err := scenario.Run(ctx, c, s.ID)
			if err != nil {
				t.Fatalf("infrastructure: %v (artifacts %s)", err, r.ArtifactDir)
			}
			if !r.Passed() {
				for _, a := range r.Assertions {
					if !a.Passed {
						t.Error(a.Message)
					}
				}
				t.Fatalf("scenario failed; artifacts %s", r.ArtifactDir)
			}
		})
	}
}

func artifactOutput(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("RBFQA_TEST_ARTIFACTS"); root != "" {
		return root
	}
	return t.TempDir()
}
