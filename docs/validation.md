# Validation record

Implementation environment: macOS arm64, Go 1.27.1, official Bitcoin Core 32.1 release binary (RPC version 320100). The downloaded arm64 macOS archive matched the pinned official SHA-256 digest. Only managed regtest nodes were used.

## Executed successfully

- All nine CLI scenarios against actual Core 32.1, including preserved and redirected competing-chain reorganization variations.
- All nine real-node Go scenario integration tests with the race detector.
- Real HTTP merchant negative canaries: duplicate-credit detection and missing-orphan-reversal detection. Both passed after a funding-selection regression was corrected and covered by a unit test.
- Reorg variations retested with the race detector after deterministic sufficient-value UTXO selection was introduced.
- Race-enabled unit tests for exact satoshi serialization, invoice policies, idempotency, independent payments, stale replay, journal reversals, append-only constraints, atomic rollback, conflict discovery, ambiguous event revisions, HTTP/RPC errors, configuration, reports, and CLI errors.
- `go vet ./...`, `gofmt` verification, `git diff --check`, and installer shell syntax validation.
- Native macOS arm64 build and CGO-free Linux amd64 / Windows amd64 cross-builds. Cross-compilation does not establish runtime behavior on those platforms.
- Doctor against Core 32.1, including wallet creation and block generation.
- Working example HTTP bridge: actual Core → observer → HTTP → SQLite merchant → independent oracle passed normal settlement.
- Offline replay for every successful CLI report and the checked-in captured fixture. Configured HTTP-adapter replay also passed against a fresh example merchant.

Commands used (paths adapted for another machine):

```sh
go vet ./...
go test -race ./cmd/rbfqa ./internal/...
RBFQA_BITCOIND=/path/to/bitcoin-32.1/bin/bitcoind go test -race -count=1 -v -timeout 15m ./...
bin/rbfqa run all --bitcoind /path/to/bitcoin-32.1/bin/bitcoind --timeout 10m
bin/rbfqa replay fixtures/duplicate-observer-events.json
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/rbfqa
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/rbfqa
```

The first full race run exposed a small-output funding-selection issue in a negative reorg canary. It was corrected by filtering for explicit funding headroom and stable outpoint ordering. The affected real-node canaries and reorg tests were rerun successfully. This is recorded to distinguish actual validation from assumed policy behavior.

The final full race-enabled run completed successfully after the correction: all nine scenario subtests and both real HTTP canaries passed. Its log is retained locally at `artifacts/final-race-tests.log`; detailed run evidence is under `artifacts/final-integration/`. Every retained report also passed offline replay through the reference engine. Additional focused tests verified the final report rendering and per-output credit-count check.

## Unverified environments and limits

- Docker image/Compose runtime: no running local Docker daemon was available. The Dockerfile and Compose setup are provided; their runtime behavior remains unverified here.
- Hosted GitHub Actions execution: workflow authored, not submitted or observed running on GitHub.
- Linux and Windows runtime behavior: builds passed locally; Linux execution is configured in CI, and Windows is build-only validation.
- External merchant process restart: the CLI can restart its bundled reference observer, but cannot restart an arbitrary HTTP service. Integrators must orchestrate their service lifecycle at a synchronization boundary.
- No package-RBF, unconfirmed-ancestor topology, historical indexing, production networks, or arbitrary consensus/policy coverage is claimed.
