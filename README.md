# Bitcoin RBF Settlement QA

A local Go CLI that tests Bitcoin merchant accounting against real Bitcoin Core regtest transactions, mempool replacements, and competing-chain reorganizations. It targets invoice backends, deposit processors, exchanges, and checkout infrastructure that need to preserve accounting correctness when transaction identities or canonical blocks change.

The tool constructs and signs controlled P2WPKH spends, drives isolated nodes, observes inputs and receiving outputs, and compares a merchant's journal with an independent scan of the active blockchain. It is a settlement testing tool, not a wallet or payment gateway.

## Quick start

Requires Go 1.24 or newer and Bitcoin Core **32.1**. Linux and macOS (including Apple Silicon) are supported. No Docker, hosted node, paid API, or real coins are required.

```sh
git clone https://github.com/ranajahanzaib/bitcoin-rbf-settlement-qa.git
cd bitcoin-rbf-settlement-qa
# Optional: installs an official release into a new directory after checking
# a SHA-256 digest pinned in the script. Inspect the script before running it.
scripts/install-core.sh .tools/bitcoin-32.1
make build
bin/rbfqa doctor --bitcoind "$PWD/.tools/bitcoin-32.1/bin/bitcoind"
bin/rbfqa run rbf-preserve-payment --bitcoind "$PWD/.tools/bitcoin-32.1/bin/bitcoind"
bin/rbfqa run all --bitcoind "$PWD/.tools/bitcoin-32.1/bin/bitcoind" --timeout 10m
```

If Core is already installed, put `bitcoind` on PATH or pass `--bitcoind`. The CLI creates fresh private temporary datadirs, descriptor wallets, and loopback RPC/P2P endpoints. It never opens an existing wallet or accepts an external Bitcoin node. Funding requires mining 101 blocks; the full suite may take several minutes on synchronous filesystems.

## Bugs it detects

- Crediting both the original transaction and its replacement.
- Crediting an unconfirmed transaction or one below the configured threshold.
- Treating a redirected replacement as a merchant payment.
- Settling the original amount after the winning transaction reduced it.
- Keeping an orphaned credit after a chain reorganization.
- Duplicating credits on repeated notifications, reconciliation, or restart.
- Losing the invoice association when the confirmed transaction has a different ID.

Two deliberately faulty HTTP merchants in integration tests demonstrate detection of duplicate credits and missing orphan-credit reversals. Their expected failures are successful tests of the checker.

## Scenario catalog

| ID                          | Test                                                              |
| --------------------------- | ----------------------------------------------------------------- |
| `normal-confirmation`       | 50,000 sats, threshold enforcement, repeated reconciliation       |
| `rbf-preserve-payment`      | Same inputs, higher fee, same merchant amount                     |
| `rbf-payment-redirect`      | Replacement pays another destination; merchant gets zero          |
| `rbf-amount-reduction`      | 30,000 received against a 50,000-sat invoice                      |
| `rbf-insufficient-fee`      | Core rejects an equal-fee conflicting spend; original settles     |
| `rbf-replacement-chain`     | Three successive versions with persisted observer restart         |
| `rbf-nonsignaling-original` | Final input sequence does not prevent modern full-RBF             |
| `reorg-competing-spend`     | Two isolated real branches; both preserved and redirected winners |
| `duplicate-observer-events` | Duplicate delivery, old revisions, restart, and replay            |

See [scenario details](docs/scenarios.md) for preconditions and assertions. Each run starts clean. Reorg scenarios deliberately use a one-confirmation policy to expose reversal behavior; other scenarios default to two confirmations.

## Architecture and accounting

```text
Go scenario → managed Core node(s) → signed transactions / active blocks
                                      ↓                  ↓
                               full-snapshot observer   independent block oracle
                                      ↓                  ↓
                             reference or HTTP merchant → invariant checker
                                      ↓                  ↓
                                  SQLite journal       JSON / Markdown evidence
```

The reference merchant stores invoices, observed transaction metadata, spent outpoints, conflicts, receiving outputs, block hashes, checkpoints, and processed events in SQLite. An eligible `txid:vout` creates an append-only credit. Losing eligibility creates a compensating reversal. A genuinely independent payment is a separate output and can contribute additional value. All accounting and checkpoint changes commit in one SQLite transaction.

Every checkpoint compares journal amounts and invoice balance with active-chain outputs independently retrieved from Core. Full snapshot scans are intentionally suited to small, controlled regtest chains. This is not a historical chain indexer.

Read [architecture](docs/architecture.md), [settlement model](docs/settlement-model.md), and [Core compatibility](docs/bitcoin-core.md).

## CLI

```sh
rbfqa init                                  # create rbfqa.yaml, never overwrite
rbfqa list
rbfqa doctor --config rbfqa.yaml
rbfqa run normal-confirmation --config rbfqa.yaml
rbfqa run all --timeout 10m --output artifacts
rbfqa report artifacts/RUN/report.json --format markdown
rbfqa replay artifacts/RUN/report.json
```

Flags can precede or follow the positional argument. Configuration is loaded only with `--config`; there is no implicit config search. `--help` is available on each command.

| Flag                                | Meaning                                                        |
| ----------------------------------- | -------------------------------------------------------------- |
| `--config FILE`                     | Strict YAML configuration; unknown keys are errors             |
| `--bitcoind PATH`                   | Core 32.1 binary                                               |
| `--adapter reference\|URL`          | SQLite reference or explicitly selected test-only HTTP service |
| `--output DIR`                      | Run artifacts, default `artifacts`                             |
| `--format terminal\|json\|markdown` | Console representation; all run formats are saved              |
| `--timeout DURATION`                | Whole command deadline, default `3m`; `all` shares this budget |
| `--keep-artifacts`                  | Keep node datadirs and wallets in system temporary storage     |
| `--verbose`                         | Print artifact location diagnostics                            |

Exit codes: **0** passed; **1** merchant invariant/scenario assertion failed; **2** invalid configuration/unsupported environment; **3** infrastructure/execution error. Doctor classifies unsupported/missing Core as an environment error. A failed node/RPC operation during execution is never presented as a settlement defect.

See [examples/config.yaml](examples/config.yaml). Reports and evidence are retained even without `--keep-artifacts`; that flag preserves the otherwise-cleaned node resources. Preserved datadir paths appear in node logs. It does not copy wallet files into the repository.

## Reports and replay

Every run writes `artifacts/<run-id>/report.json`, `report.md`, `events.json`, `transactions.json`, node logs, and (for the reference adapter) `merchant.sqlite`.

A passing terminal summary looks like:

```text
PASS rbf-preserve-payment — Core 320100 · 59/59 assertions · artifacts/RUN
```

A defective merchant produces messages such as:

```text
INV-001 FAILED: No duplicate effective credit; invoice=... expected=50000 actual=100000 tip=... outputs=...
FAIL duplicate-observer-events — Core 320100 · ... assertions · artifacts/RUN
```

A [captured failing example](docs/example-failure.md) shows the duplicate-credit canary and its violating journal.

Reports include exact transaction versions, signed regtest hex, fees/vsize, acceptance decisions and reasons, chain snapshots, disconnected hashes, merchant snapshots, journal entries, and checkpoint-specific assertion failures. RPC passwords, cookies, and adapter bearer tokens are excluded. Protect artifacts as test data; logs include temporary paths and test addresses.

Replay reconstructs accounting in a fresh temporary SQLite database, applies recorded observations, delivers the same history again, and checks final balances against the captured independent oracle. It is **offline event/accounting replay**, not rebroadcasting old raw transactions onto a new funding chain. Fresh wallets change addresses, txids, and hashes: scenarios guarantee logical repeatability, not byte-for-byte identical chains.

## Test your backend

Implement the opt-in [HTTP adapter contract](docs/adapter-protocol.md). An arbitrary production merchant API is not assumed compatible.

```sh
# Terminal 1: working example bridge, with its wallet-free SQLite DB outside source.
go run ./examples/merchant-adapter --db /tmp/rbfqa-example.sqlite
# Terminal 2:
bin/rbfqa run rbf-preserve-payment --adapter http://127.0.0.1:8787
```

Use an isolated test tenant. The URL explicitly selects a test service; HTTPS is required except on loopback. Set `RBFQA_ADAPTER_TOKEN` for bearer authentication. The adapter must expose invoice/payment state and a real journal, honor event revision ordering, and reconcile synchronously before responding. The runner independently verifies the reported result.

## Docker

```sh
docker compose -f docker/compose.yaml build
docker compose -f docker/compose.yaml run --rm qa doctor
docker compose -f docker/compose.yaml run --rm qa
```

The image builds the CLI and installs checksum-pinned Core 32.1. Runtime has no external network and retains artifacts in the `evidence` volume. To export reports, run a named container without `--rm`, then `docker cp CONTAINER:/work/artifacts ./docker-artifacts`. Native installation remains the primary local path.

## Development

```sh
make check
RBFQA_BITCOIND=/absolute/path/to/bitcoind make integration
```

Without `RBFQA_BITCOIND`, real-node integration tests explicitly skip; unit tests never claim to validate Core behavior. GitHub Actions runs formatting, vet, race tests, native/cross builds, all nine real scenarios, HTTP fault canaries, and offline replay, and uploads run evidence. See [contributing](docs/contributing.md).

## Guarantees and limitations

Passing proves the implemented checks held at the captured checkpoints, for these controlled confirmed-input P2WPKH topologies and the configured adapter. It does not prove immunity to every double spend, reorganization, consensus failure, observer loss, or merchant bug. Confirmations are reversible policy evidence, not absolute finality.

Only Core 32.1 is accepted (RPC version `320100`). Core decides acceptance; no historical BIP125 policy engine or removed `mempoolfullrbf` option is used. No Lightning, Taproot-specific construction, package RBF, complex scripts, mainnet, hosted services, or production wallet access. Event revisions are a single trusted observer's local causal ordering, not a distributed consensus protocol. A missing transaction stays unknown unless shared-input evidence establishes conflict.

The Docker build and hosted CI need their respective environments; see [validation notes](docs/validation.md) for checks actually executed during implementation.

MIT licensed; see [LICENSE](LICENSE).
