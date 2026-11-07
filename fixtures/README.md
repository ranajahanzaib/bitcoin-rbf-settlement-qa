# Captured accounting fixture

`duplicate-observer-events.json` is real Bitcoin Core 32.1 regtest evidence captured on macOS arm64 during a successful scenario. Its events preserve actual output/chain identities; only the final merchant/oracle snapshot is retained to keep the fixture compact. It contains no wallet credentials.

Run `rbfqa replay fixtures/duplicate-observer-events.json` without Bitcoin Core. This exercises accounting replay against captured evidence; it is not a substitute for real-node integration. Signed transactions, if present in event metadata, are regtest-only and are not portable to a fresh funding blockchain.
