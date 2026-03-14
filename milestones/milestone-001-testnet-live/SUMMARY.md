# Milestone 001: SCAVIUM Testnet Live

Initial operational SCAVIUM testnet successfully deployed.

## Network Architecture

- Bootnodes: 2
- Validators: 11
- RPC nodes: 2
- Consensus: QBFT
- Client: Hyperledger Besu

## Network Ports

- P2P: 31303
- RPC HTTP: 18545
- RPC WS: 18546
- Metrics: 19545

## Operational Validation

The following checks were successfully completed:

- block production confirmed
- RPC connectivity validated
- peer discovery functioning
- transaction submission verified
- transaction receipts validated
- balance updates confirmed

## Observed Startup Behavior

During initial network startup, validators may temporarily enter round changes until quorum synchronization occurs.

Once validators align on the same round, block production begins normally.

## Next Milestone

Development of operational tooling:

- wallet utilities
- transaction sender
- faucet tools
- network inspection utilities