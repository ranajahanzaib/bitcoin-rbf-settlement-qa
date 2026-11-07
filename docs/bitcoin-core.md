# Bitcoin Core integration

The supported integration target is **Bitcoin Core 32.1**, RPC numeric version **320100**. Startup checks regtest chain identity, version, required RPC help capabilities, and mempool policy information. Doctor also creates a wallet and mines a block. Other releases are rejected; there is no untested compatibility fallback.

Official release: https://bitcoincore.org/en/releases/32.1/

The installer downloads only HTTPS official release archives and compares the archive with a source-pinned SHA-256 digest. Supported download platforms are Linux amd64/arm64 and macOS amd64/arm64. It refuses an existing install destination. Digests are sourced from the official 32.1 SHA256SUMS manifest; this is checksum pinning, not a local GPG signature verification ceremony.

Managed nodes use `-regtest`, loopback RPC/P2P, no seed discovery, no NAT mapping or Tor listening, `-txindex=1`, and private temporary datadirs. Descriptor wallets are Core's default. Private local regtest credentials are randomly generated and never written to reports. No external node endpoints, network selection, wallet path, or arbitrary datadir cleanup are accepted.

Transaction construction calls `listunspent`, `createrawtransaction`, `signrawtransactionwithwallet`, and `decoderawtransaction`. Every payment uses explicit input selection and integer-satoshi output/fee accounting. RPC Bitcoin decimal amounts are parsed exactly; exponent and sub-satoshi inputs are rejected. Acceptance calls `testmempoolaccept` before `sendrawtransaction` and records the returned decision. A failed transport is distinct from a valid policy rejection.

Modern full-RBF is authoritative. No removed `-mempoolfullrbf` option is supplied; no deprecated `bip125-replaceable` or `fullrbf` fields are consulted. The tests do not reproduce the historical BIP125 rule set or implement cluster mempool policy. Large fee increments simplify the confirmed-input tests, but acceptance still depends on Core. The effective version and mempool policy snapshot are in reports.

The observer uses `getblockchaininfo`, `getblockhash`, `getblock` verbosity 2, `getrawmempool`, and `getrawtransaction`; the oracle separately retrieves active blocks. Node connection management uses `setnetworkactive`, `getpeerinfo`, and `addnode`. Mining uses bounded batches of `generatetoaddress`, with RPC deadlines and a command deadline. Polling waits for explicit conditions rather than treating elapsed sleep as success.

## Troubleshooting

- **Binary missing:** install 32.1 and use an absolute `--bitcoind` path.
- **Unsupported version:** inspect `rbfqa doctor`; use the pinned release rather than adding old response-field assumptions.
- **Loopback denied:** run in an environment that permits listening on localhost; container networking can be disabled externally because the nodes communicate over loopback inside one container.
- **Startup/RPC timeout:** inspect artifact node logs; increase `--timeout` for slow disks. Each RPC has a bounded transport timeout; initial funding is split into ten-block batches.
- **Unexpected replacement rejection:** inspect `mempool_acceptance` and transaction fee/vsize. Do not ignore the reason or replace a real test with a mocked success.
- **No Docker daemon:** use the native binary. Docker is optional.
- **Inspect a failed chain:** rerun with `--keep-artifacts`; read the preserved datadir path in logs. Cleanup removes only tool-created temporary directories.
