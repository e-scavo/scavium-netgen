#!/bin/bash

./scavium-netgen init \
  --base /opt/scavium/testnet \
  --nodes-file ./nodes.json \
  --besu /usr/local/bin/besu \
  --inventory-out /opt/scavium/testnet/inventory.json \
  --hosts-out /opt/scavium/testnet/hosts.generated