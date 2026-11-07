# Example: duplicate-credit detection

Captured from a real Bitcoin Core 32.1 regtest run through the deliberately faulty HTTP merchant in `test/integration/faults_test.go`. This is an expected negative canary result; a correctly working reference merchant passes the scenario.

- Scenario: `duplicate-observer-events`
- Checkpoint: `settled`
- Invariant: `INV-001` failed
- Expected recognized value: 50000 sats
- Actual merchant balance: 100000 sats
- Exit code: 1 (settlement defect)

The faulty merchant appends another effective credit when it receives confirmed observations. The checker independently reads the active-chain payment output and detects that journal credits exceed that canonical value. The bug is accounting duplication, not an additional independent payment.

## Violating journal at the checkpoint

| Entry | Output                                                               | Delta (sats) |
| ----- | -------------------------------------------------------------------- | -----------: |
| 1     | `51a353b04f8e6ec1afa02e1e91ce7d3c9de6b400cbfa6093280076804d9453c1:0` |        50000 |
| 10000 | `51a353b04f8e6ec1afa02e1e91ce7d3c9de6b400cbfa6093280076804d9453c1:0` |        50000 |

## Reproduce the detection

```sh
RBFQA_BITCOIND=/path/to/bitcoin-32.1/bin/bitcoind \
  go test -race -v ./test/integration -run 'TestRealCoreDetectsFaultyHTTPMerchants/duplicate'
```

The Go test passes only when it detects the expected INV-001 failure. Set `RBFQA_TEST_ARTIFACTS` to a directory to retain its full JSON/Markdown report, transaction versions, acceptance results, chain evidence, event timeline, and merchant snapshots.
