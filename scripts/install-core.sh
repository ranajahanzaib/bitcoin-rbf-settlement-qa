#!/bin/sh
# Download only official 32.1 release archives and verify a pinned digest.
set -eu
version=32.1
case "$(uname -s)-$(uname -m)" in
 Linux-x86_64) platform=x86_64-linux-gnu; digest=61545c17c3120fb7412f896d248b11993af04ec8817aad3649c55b2afb044963 ;;
 Linux-aarch64|Linux-arm64) platform=aarch64-linux-gnu; digest=7ed37c7373eb6aa3e3f72842596a1afeba1b223717033cade9842b25773e1922 ;;
 Darwin-arm64) platform=arm64-apple-darwin; digest=cd3dcdacb4ff861e862b2d7df4786ecef66472e2468dca1673a70dd761e064e7 ;;
 Darwin-x86_64) platform=x86_64-apple-darwin; digest=53b1e6ffd92843ca82ef4a737d7a53cb0a9fab15816a417a41f12fc954071447 ;;
 *) echo 'Unsupported host; install official Bitcoin Core 32.1 manually.' >&2; exit 2 ;;
esac
# The destination must be new: never overwrite or remove a user directory.
destination=${1:-"./.tools/bitcoin-32.1"}
if [ -e "$destination" ]; then echo "Destination already exists: $destination" >&2; exit 2; fi
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
archive="bitcoin-$version-$platform.tar.gz"
curl --fail --location --proto '=https' --tlsv1.2 "https://bitcoincore.org/bin/bitcoin-core-$version/$archive" -o "$work/$archive"
if command -v sha256sum >/dev/null 2>&1; then actual=$(sha256sum "$work/$archive" | cut -d ' ' -f 1); else actual=$(shasum -a 256 "$work/$archive" | cut -d ' ' -f 1); fi
if [ "$actual" != "$digest" ]; then echo 'Checksum mismatch; refusing to extract.' >&2; exit 3; fi
tar -xzf "$work/$archive" -C "$work"
mkdir -p "$(dirname "$destination")"
mv "$work/bitcoin-$version" "$destination"
printf 'Verified Bitcoin Core %s installed: %s/bin/bitcoind\n' "$version" "$destination"
