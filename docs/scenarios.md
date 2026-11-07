# Scenarios

All nine scenarios require an exact supported Core 32.1 binary, descriptor wallets, regtest RPC capabilities, loopback networking, and writable temporary storage. Each starts from a fresh node with 101 mined blocks and an explicitly chosen mature confirmed P2WPKH UTXO. The invoice is 50,000 sats. Fees are explicit absolute satoshi values, normally 2,000 / 20,000 / 40,000 for A / B / C. Core's acceptance result is authoritative: unexpected rejection is an execution failure with its actual reason recorded.

## normal-confirmation

Broadcast A; verify no finalized credit in mempool. Mine one block; enforce the configured threshold. Mine remaining confirmations, compare with the independent oracle, and reconcile again. Exactly one journal credit and 50,000 net sats must remain.

## rbf-preserve-payment

Observe A, replace using the same confirmed input with B, reduce change for a higher fee, preserve the merchant output. `testmempoolaccept` must allow B. The mempool must contain B and exclude A. Mine B and require one net 50,000-sat credit. Retain A and shared-input history.

## rbf-payment-redirect

Replace A with B paying another controlled regtest address. Observe B through A's tracked input even though B has no watched merchant output. Mine B. Final invoice is unpaid with no finalized merchant credit.

## rbf-amount-reduction

Replace the 50,000-sat output with 30,000 sats. Mine to threshold. Recognize exactly 30,000, retain 20,000 outstanding, and mark the invoice partially paid.

## rbf-insufficient-fee

Construct a different transaction B with the same absolute fee as A and a changed merchant amount. Core must reject it without a hardcoded reason-string comparison. Record its structured acceptance result and actual rejection reason. B is not broadcast or inserted into the mempool; A remains and ultimately receives the only credit.

## rbf-replacement-chain

A → B → C, each accepted with a substantial fee increase. Restart the reference observer/SQLite engine after B, reload the durable checkpoint, then broadcast C. All three versions remain traceable. Only C settles. The external HTTP merchant must manage its own process lifecycle; the CLI cannot restart an arbitrary remote service.

## rbf-nonsignaling-original

A has sequence `0xffffffff`, which does not signal historical opt-in replacement. B must be evaluated and accepted by actual modern full-RBF policy, then settle once. Signaling never grants finality.

## reorg-competing-spend

Runs two variations with fresh invoices. Start node B and synchronize a shared funding chain with A. Sign both conflicts on A before disconnecting. Set networking inactive on both nodes and wait until peer lists are empty. Broadcast A and B into different mempools, mine both branches, and observe a one-confirmation credit on A. Extend B two additional blocks so its cumulative work wins, reactivate networking, reconnect, and wait for identical best hashes. Verify A's credited block is noncanonical and the observer records disconnected hashes.

For the preserved-payment variant, reverse A's 50,000 credit and credit canonical B for 50,000. For the redirected variant, reverse A and leave zero. Reopen the reference engine and reconcile again to verify durable recovery. The simple regtest branches have the same per-block work; extra blocks give strictly greater cumulative work.

## duplicate-observer-events

Capture real mempool and confirmed observations, repeat the latest delivery, reopen the reference engine, then replay all older snapshots in order. Confirmed state must not regress to an older mempool snapshot, and journal balance/credit count remain unchanged. CLI offline replay additionally applies the entire captured history twice to a fresh database.

## Negative canaries and limits

`test/integration/faults_test.go` runs actual scenarios through test-only HTTP merchants that duplicate confirmed credits or omit reversals from their exposed journal. The duplicate canary must fail INV-001 and the reorg canary INV-007, with settlement exit code 1. Expected detection passes the Go test.

These workflows use isolated confirmed inputs, no unconfirmed ancestors, and no package-RBF topology. Their logical order is deterministic; generated keys, txids, block times, and hashes are not identical across fresh runs. Real Core integration tests explicitly skip when the binary is unavailable.
