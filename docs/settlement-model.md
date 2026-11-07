# Settlement model

An invoice has an expected amount, unique receiving address, and positive confirmation threshold. Transaction status and invoice status are distinct. Several independent payments may fund one invoice, while several conflicting versions of a payment cannot all become canonical.

Transactions are `mempool`, `confirmed`, `conflicted`, or `unknown`. Shared `txid:vout` inputs establish conflict evidence, including replacements without merchant outputs. Mempool disappearance alone establishes neither replacement nor failure. Raw metadata retains the last known block context; the explicit current status determines eligibility.

Invoice statuses are `unpaid`, `observed`, `partially_paid`, and `settled`. A below-threshold received amount is provisional and appears as pending. Eligible partial payments are credited at their actual value; they do not fully settle an underfunded invoice. Outstanding value is max(expected minus net balance, zero). Reorgs and conflicts are transaction/chain history rather than mutually exclusive invoice states.

Accounting uses output identities `txid:vout`. Only active-chain outputs at the invoice's threshold are eligible. Journal delta is a signed integer number of satoshis. Each credit has a unique idempotency key. A reversal references one credit ID, has the opposite delta, and can occur only once for that credit. A SQL trigger prohibits journal update or deletion. If an output becomes canonical again after reversal, a new credit generation may be appended; historical gross credits can exceed the invoice while net recognized value remains correct.

Net balance is the sum of journal deltas. Gross credits sum positive deltas; reversals sum the magnitude of negative deltas. Pending value excludes eligible outputs. These values must not be conflated. Default settlement requires two confirmations; the reorg tests use one and still require reversals after disconnection.

Snapshot reconciliation, invoice changes, entries, processed-event markers and hash checkpoints are transactional. WAL mode, foreign keys, FULL synchronous durability, uniqueness constraints, and a bounded busy timeout are enabled. A restart loads the last durable snapshot. Older revisions and repeated event IDs have no effect. Equal revisions with differing tips are rejected as ambiguous. Missing authoritative chain context requests reconciliation through an error; it never guesses a financial update.

Revisions describe trusted sequential observer snapshots. HTTP integrations must preserve that ordering and complete state semantics. They must not treat an untrusted arbitrary event as proof of canonicality. The runner's independent Core oracle remains the final test authority.
