# Test-only HTTP adapter protocol v1

This is an explicitly enabled QA contract for an isolated test tenant. It is not a production payment API or presumed native interface of another product. Select the origin using `--adapter URL`; HTTPS is required except for literal loopback IPs or localhost. No redirects, URL userinfo, query credentials, or origin paths are allowed. `RBFQA_ADAPTER_TOKEN` (or configured `adapter_token_env`) supplies an optional bearer token. Each request has a 15-second transport timeout within the command deadline; response bodies are bounded to 16 MiB.

Use `Content-Type: application/json`. Successful POSTs return any 2xx status (204 recommended); GETs return 200 with the specified JSON. Non-2xx, malformed JSON, missing invoice identity, or inconsistent financial state cause a failed run. Do not wrap responses in an extra `data` object. All work must finish before responding: an asynchronous acknowledgment is insufficient.

## POST /qa/invoices

```json
{
  "id": "run-payment",
  "address": "bcrt1q...",
  "expected_sats": 50000,
  "required_confirmations": 2,
  "status": ""
}
```

Register this exact receiving address and policy under the provided ID. The runner generates unique run/invoice IDs. The service must isolate QA invoices from production data. Duplicate IDs should either be verified identical or rejected; they must never silently change an existing invoice policy.

## POST /qa/events

The body is a `model.Event`, defined in `internal/model/model.go`:

```json
{
  "id": "content-digest",
  "revision": 3,
  "tip": "active-tip-hash",
  "height": 102,
  "blocks": ["genesis-hash", "...", "active-tip-hash"],
  "disconnected": ["orphaned-block-hash"],
  "transactions": [
    {
      "txid": "transaction-id",
      "vin": [{ "txid": "previous-id", "vout": 0, "sequence": 4294967293 }],
      "vout": [
        { "n": 0, "value": 0.0005, "scriptPubKey": { "address": "bcrt1q..." } }
      ],
      "vsize": 141,
      "blockhash": "active-block-hash",
      "height": 102,
      "confirmations": 1,
      "status": "confirmed",
      "fee_sats": 0
    }
  ]
}
```

The example uses ellipses for readability; actual `blocks` is a complete height-indexed hash list. Each event is a complete authoritative snapshot of non-coinbase active-chain transactions plus current mempool transactions. Coinbase transactions are omitted by the observer because generated invoices never receive mining rewards. Absent previously observed transactions become unknown unless shared-input conflict evidence exists. An output's decimal `value` follows Bitcoin Core's **BTC JSON convention** and must be parsed exactly, without float arithmetic. All fields explicitly suffixed `_sats` are integer satoshis. `fee_sats` is zero when the snapshot does not supply transaction-factory metadata. The report's separate transaction list supplies actual constructed fees.

`revision` is a monotonic causal sequence scoped to this observer run; `id` is content-derived. Duplicate IDs and older revisions must not mutate financial state. A revision collision with inconsistent chain context must request reconciliation/fail rather than guess. Confirmed outputs are eligible only when the block hash matches `blocks[height]` and depth meets policy. Track shared inputs even when a replacement no longer pays the watched address. Commit credit/reversal journal entries and processed snapshot context atomically.

## POST /qa/reconcile

```json
{ "revision": 3, "tip": "active-tip-hash" }
```

Complete settlement reconciliation against the last delivered snapshot before replying. This is a synchronization barrier, not permission to trust a merchant-supplied success flag. The runner queries state and independently scans the chain after this call.

## GET /qa/invoices/{id}

Return `model.State`:

```json
{
  "invoice": {
    "id": "run-payment",
    "address": "bcrt1q...",
    "expected_sats": 50000,
    "required_confirmations": 2,
    "status": "settled"
  },
  "balance_sats": 50000,
  "pending_sats": 0,
  "outstanding_sats": 0,
  "gross_credits_sats": 50000,
  "reversals_sats": 0,
  "entries": [
    {
      "id": 1,
      "idempotency_key": "credit:tx:0:3",
      "invoice_id": "run-payment",
      "output": "tx:0",
      "delta_sats": 50000
    }
  ],
  "transactions": [],
  "revision": 3,
  "tip": "active-tip-hash"
}
```

`transactions` must expose retained observed payment/conflict records using `model.Tx`. The empty list above is only a schema illustration. Include current status, inputs, outputs, and last known block context. Invoice states supported by the reference contract are `unpaid`, `observed`, `partially_paid`, and `settled`. Map a richer backend domain to these semantics.

## GET /qa/invoices/{id}/ledger

Return the authoritative append-only entry array, without an envelope:

```json
[
  {
    "id": 1,
    "idempotency_key": "credit:tx:0:3",
    "invoice_id": "run-payment",
    "output": "tx:0",
    "delta_sats": 50000
  },
  {
    "id": 2,
    "idempotency_key": "reverse:1",
    "invoice_id": "run-payment",
    "output": "tx:0",
    "delta_sats": -50000,
    "reversal_of": 1
  }
]
```

IDs must be unique and ordered, keys unique, and reversal references point to the credited entry. Expose actual accounting history, not a derived success indicator. The runner checks net sums, effective output credits, reversal validity, expected amount, threshold, invoice metadata, and current tip. Reports retain violating entries for investigation.

## Working bridge

`go run ./examples/merchant-adapter --db /tmp/rbfqa-example.sqlite` runs a loopback-only SQLite-backed implementation. Use it as contract guidance, then replace its engine calls with integration code for your merchant backend. Protect a remote test service with TLS and authentication; selecting its URL explicitly authorizes testing that environment.

The CLI restarts its own reference engine during restart scenarios; it cannot stop/start an arbitrary external service. To test external process recovery, orchestrate that service's restart at your bridge's chosen synchronization boundary. Offline replay may target a fresh reference database or an explicitly configured empty test adapter.
