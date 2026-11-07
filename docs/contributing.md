# Contributing

Use Go 1.24+ and Core 32.1. Run `make check` and `RBFQA_BITCOIND=/absolute/path/to/bitcoind make integration`. The latter executes actual nodes, all scenarios, and two faulty HTTP merchant canaries. Normal Go runs skip node integration when the environment variable is absent. Never describe a skip as a successful integration result.

Keep money in integer satoshis. Parse Core BTC decimals exactly. Domain snapshots use Core-style BTC decimal output `value` fields and explicitly named integer `_sats` fields; preserve their distinction in JSON tests. Core decides mempool policy; do not add historical BIP125 assumptions or acceptance string matching.

Add scenarios to `internal/scenario` with metadata, isolated setup, explicit observation checkpoints, expected workflow assertions, and independent Core oracle comparisons. Record evidence before cleanup. A scenario is validated only after successful real-node execution. Add negative canaries when a new invariant could otherwise silently accept a defective merchant.

Use bounded contexts and condition polling. Do not introduce arbitrary sleeps, hosted services, production node configuration, unsafe cleanup paths, or fixture tests masquerading as Core integration. Preserve journal history: reversals are entries, never deletes. Accounting and checkpoint transitions must remain atomic.

Reports should explain exact expected/actual values, output identity, chain context, and checkpoint. Do not serialize credentials. Document policy/version limitations and changes to the adapter wire contract.

Pull requests should state the concrete behavior changed and the checks actually executed. MIT contributions follow the repository license and Code of Conduct.
