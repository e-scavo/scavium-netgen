# scavium-netgen

Network generator for SCAVIUM, an EVM-compatible blockchain network based on Hyperledger Besu and QBFT consensus, implemented in Go.

This tool bootstraps a full private blockchain network with reproducible configuration and operational documentation.

## Purpose

scavium-netgen creates the full structure required to deploy a SCAVIUM network:

- bootnodes
- validators
- RPC nodes
- QBFT genesis
- static node topology
- per-node configuration
- systemd service templates
- network inventory
- hosts file
- operational accounts

The goal is to provide a repeatable and deterministic network bootstrap process.

## Features

The generator creates:

- node identity keys
- operational accounts (faucet, deployer, treasury, etc.)
- qbft_validators.json
- extraData for QBFT
- genesis.json
- static-nodes.json
- config.toml per node
- systemd template
- inventory file
- hosts file
- accounts inventory

## Requirements

- Go
- Hyperledger Besu
- Linux servers running systemd
- network connectivity between nodes

## Subcommands

### init

Initializes a complete network from scratch.

Generates:

- node directories
- node keys
- operational accounts
- genesis
- static nodes
- configs
- systemd templates
- inventory
- documentation

### regen

Regenerates derived artifacts such as:

- configs
- static nodes
- documentation
- inventory

### inventory

Generates only documentation artifacts:

- node inventory
- hosts file
- accounts inventory
- network documentation

## Example

```bash
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
  --hosts-out /opt/scavium/testnet/hosts.generated
```

## Generated Structure

```text
/opt/scavium/testnet/

accounts/
network/
nodes/

genesis.json  
qbft_validators.json  
extraData.txt  
inventory.json  
hosts.generated  
README.generated.md  
scavium-besu@.service
```

## Operational Accounts

The generator creates operational accounts separate from node identities:

- faucet
- deployer
- treasury
- ops
- tester_01
- tester_02

These accounts are allocated in the genesis and used for operational testing.

## Default Ports

- P2P: 31303
- RPC HTTP: 18545
- RPC WS: 18546
- Metrics: 19545

## Recommended Startup Order

1. Bootnodes
2. Validators
3. RPC nodes

## Known Behavior During First Startup

When a QBFT network starts for the first time, validators may temporarily enter round changes before block production begins.

If blocks are not produced immediately:

- verify peer connectivity
- wait for consensus convergence
- if necessary restart validators in a short window

Once quorum is reached, block production begins normally.

## Project Status

Current milestone documented in:

`milestones/milestone-001-testnet-live/`

## Components

### scavium-faucet

Public token-distribution service with web UI, REST API, rate-limiting, captcha, and admin API.

- Source: [`cmd/scavium-faucet/`](cmd/scavium-faucet/)
- Documentation: [`docs/scavium-faucet/`](docs/scavium-faucet/)

## Roadmap

Next development steps:

- operational tooling
- wallet utilities
- transaction sender
- transaction receipt tools