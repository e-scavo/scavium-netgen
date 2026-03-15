#!/usr/bin/env bash
set -euo pipefail

RPC="${1:-http://191.102.248.175:18845}"
PRIVATE_KEY="${2:-}"
WALLETS_FILE="${3:-wallets.txt}"
VALUE_WEI="${4:-1000000000000000000}"   # 1 SCV default
SLEEP_SECS="${5:-0.2}"

if [[ -z "$PRIVATE_KEY" ]]; then
    echo "Usage:"
    echo "  $0 <rpc-url> <faucet-private-key> <wallets-file> [value-wei] [sleep-seconds]"
    exit 1
fi

if [[ ! -f "$WALLETS_FILE" ]]; then
    echo "ERROR: wallets file not found: $WALLETS_FILE"
    exit 1
fi

echo "======================================"
echo "SCAVIUM WALLET FUNDING SCRIPT"
echo "RPC:              $RPC"
echo "Wallet list:      $WALLETS_FILE"
echo "Amount per wallet:$VALUE_WEI wei"
echo "Sleep between tx: $SLEEP_SECS s"
echo "======================================"

COUNT=0

while read -r WALLET; do
    WALLET="$(echo "$WALLET" | xargs)"

    if [[ -z "$WALLET" ]]; then
        continue
    fi

    echo ""
    echo "Funding wallet: $WALLET"

    ./bin/tx-send \
        "$RPC" \
        "$PRIVATE_KEY" \
        "$WALLET" \
        "$VALUE_WEI"

    COUNT=$((COUNT+1))
    sleep "$SLEEP_SECS"

done < "$WALLETS_FILE"

echo ""
echo "======================================"
echo "Funding complete"
echo "Wallets processed: $COUNT"
echo "======================================"