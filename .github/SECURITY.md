# Security policy

Report vulnerabilities through the repository's private GitHub Security Advisories when available. Include a reproduction and impact; do not include real wallet credentials or production data.

This project supports managed Bitcoin Core 32.1 regtest nodes only. Never configure an adapter against a production merchant tenant. Artifacts contain test transactions, chain evidence, addresses, and local paths. RPC cookies, passwords, and bearer tokens must not appear in published reports.

Download digests are pinned in `scripts/install-core.sh`. Changes to release sources or hashes require review against official Bitcoin Core release evidence. Do not disable checksum verification to bypass a failed download.
