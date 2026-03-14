#!/bin/bash

./scavium-netgen init \
  --base /opt/scavium/testnet \
  --chain-name SCAVIUM \
  --network testnet \
  --gateway 191.102.248.190 \
  --besu /usr/local/bin/besu \
  --generate-extradata true \
  --generate-systemd true \
  --generate-accounts true \
  --verbose true \
  --debug false \
  --inventory-out /opt/scavium/testnet/inventory.json \
  --accounts-out /opt/scavium/testnet/accounts/accounts.json \
  --hosts-out /opt/scavium/testnet/hosts.generated \
  --p2p-port 31303 \
  --rpc-port 18545 \
  --ws-port 18546 \
  --metrics-port 19545
