#!/usr/bin/env bash
set -euo pipefail

COUNT="${1:-10}"
OUT_DIR="${2:-./stress-artifacts}"

mkdir -p "$OUT_DIR"

WALLETS_FILE="$OUT_DIR/wallets.txt"
KEYS_FILE="$OUT_DIR/stress_keys.txt"

: > "$WALLETS_FILE"
: > "$KEYS_FILE"

echo "======================================"
echo "SCAVIUM STRESS WALLET GENERATOR"
echo "Wallet count: $COUNT"
echo "Output dir:   $OUT_DIR"
echo "======================================"

for ((i=1; i<=COUNT; i++)); do
    OUTPUT="$(./bin/wallet-new)"

    ADDRESS="$(echo "$OUTPUT" | awk -F': ' '/^Address:/ {print $2}')"
    PRIVATE_KEY="$(echo "$OUTPUT" | awk -F': ' '/^PrivateKey:/ {print $2}')"

    if [[ -z "$ADDRESS" || -z "$PRIVATE_KEY" ]]; then
        echo "ERROR: could not parse wallet-new output for wallet $i"
        exit 1
    fi

    echo "$ADDRESS" >> "$WALLETS_FILE"
    echo "$PRIVATE_KEY" >> "$KEYS_FILE"

    echo "[$i/$COUNT] generated $ADDRESS"
done

echo ""
echo "Done."
echo "Addresses:    $WALLETS_FILE"
echo "Private keys: $KEYS_FILE"
echo "======================================"