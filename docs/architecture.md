# Architecture

`cmd/rbfqa` parses commands and deadlines. `internal/config` validates a deliberately small strict YAML configuration. `internal/core` owns RPC calls, isolated node subprocesses, descriptor wallets, explicit confirmed-input transactions, mempool acceptance, and peer synchronization.

`internal/scenario` runs fixed Go workflows. Each run gets a fresh node and merchant database. The generator never inserts ledger entries. It supplies transactions to Core; the observer independently discovers watched outputs and tracked spent inputs. Reorganization tests sign competing spends before isolation, disable networking on both nodes, mine separate branches, then reconnect to the strictly longer branch. Original credited block headers must report negative confirmations after convergence.

`internal/observer` snapshots the complete small active regtest chain and current mempool. It checks network identity and tip stability. Block hashes identify the common ancestor by comparing checkpoint prefixes; the differing suffix records disconnected blocks. Every snapshot has a content-derived event ID and a monotonically increasing observer revision. Restart reloads the event/checkpoint from SQLite.

`internal/settlement` reconciles a complete authoritative snapshot in one SQL transaction: mark vanished transactions unknown, discover watched outputs and shared-input conflicts, update canonical eligibility, reverse orphaned credits, credit newly eligible outputs, update invoice states, and replace block checkpoints. The revision and processed-event record commit together. This makes crashes before commit roll back and crashes after commit safe to retry.

The `internal/adapter` interface has create, apply/reconcile, and state methods. The direct SQLite implementation and HTTP bridge share normalized models. A bridge may translate snapshots to a merchant's native API, but must return its actual financial journal. The QA contract is opt-in and restricted to explicitly selected test environments.

`internal/invariant` has a separate oracle that walks active Core blocks and calculates watched output value and confirmation depth. It does not read the merchant DB or derive expectations from a merchant success flag. Ten invariant families compare active effective credits, reversals, output identity, amounts, checkpoints, and invoice reconciliation. Scenario assertions additionally check exact workflow outcomes, mempool displacement, orphaned block identity, and expected credit counts.

`internal/report` writes machine-readable evidence, a Markdown investigation report, and normalized replayable snapshots. Artifacts survive node cleanup. Node log copies are made before shutdown; no wallet files or RPC authentication data are serialized.

The snapshot algorithm is intentionally O(chain length) per checkpoint. It favors understandable recovery semantics for small deterministic regtest chains over production indexing throughput. Concurrent chain changes produce retry/error rather than guessed settlement. There is one sequential observer source; revisions must not be reused across divergent chains.
