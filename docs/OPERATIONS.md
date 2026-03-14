# OPERATIONS

Operational procedures for deploying and maintaining a SCAVIUM network.

## Network Deployment

1. Generate the network using:

```bash
scavium-netgen init
```

2. Distribute `/opt/scavium/testnet` to each node host.

3. Install the systemd template:

```bash
cp scavium-besu@.service /etc/systemd/system/
systemctl daemon-reload
```

4. Start nodes in this order: bootnodes, validators, RPC nodes.

## Quick Health Checks

### Current Block Number

```bash
curl -s -X POST http://127.0.0.1:18545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

### Peer Count

```bash
curl -s -X POST http://127.0.0.1:18545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'
```

## Known Issue: Round Changes During First Startup

### Symptoms

- peers are visible
- no blocks are produced
- logs show `RoundChangeManager`

### Suggested Actions

- wait for network convergence
- verify validator connectivity
- if necessary, restart validators in a short window

After quorum is reached, block production should start automatically.

## Full Network Reset

To completely regenerate the network:

1. stop all nodes
2. delete the network directory

```bash
rm -rf /opt/scavium/testnet
```

3. regenerate the network

```bash
scavium-netgen init
```

4. redistribute node directories
5. restart nodes

## Best Practices

- keep node identity keys separate from operational accounts
- never store private keys in Git repositories
- document genesis and validator set
- store milestone artifacts for traceability