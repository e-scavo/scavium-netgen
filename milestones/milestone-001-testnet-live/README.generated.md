# SCAVIUM / testnet

- Chain ID: 1987374788
- Gateway: 191.102.248.190
- Base dir: /opt/scavium/testnet
- Genesis: /opt/scavium/testnet/genesis.json
- Faucet alloc: 0x5FA808aD3C53c8b7554bcc033F0e10c2b039bBD4
- Deployer alloc: 0xaee0b63ad1e319349cCB72b8c03b0cE522B38F16
- extraData auto-generated: true

## Default Ports

- P2P: 31303
- RPC HTTP: 18545
- RPC WS: 18546
- Metrics: 19545

## Nodes

| Name | Role | IP | Address |
|---|---|---|---|
| B01 | bootnode | 191.102.248.161 | 0x9F24540271ED14623F72Fea654b8997eF5a0C76F |
| B02 | bootnode | 191.102.248.162 | 0xADCaC4bE93b59D1Cd67FDBc98523233c48ccbB22 |
| V01 | validator | 191.102.248.163 | 0x02490E734B3881927B3281174d1958683ada361A |
| V02 | validator | 191.102.248.164 | 0x62fB4a8d9f39940aC33d2777b162d72EF3319Fd2 |
| V03 | validator | 191.102.248.165 | 0x75831BeD1bc70A5421930d9Fc8908E3dF075bB48 |
| V04 | validator | 191.102.248.166 | 0x1a9d63c69946eb14498EacdD4EbaA7Ed2D7cc81C |
| V05 | validator | 191.102.248.167 | 0xcB29a01c2CFBC0F06888C6a0B47ad19cb7a13782 |
| V06 | validator | 191.102.248.168 | 0x1535B4E88EE1ee9bB01F0eBBEF167aDB42D98849 |
| V07 | validator | 191.102.248.169 | 0x69C9744eC42014CE2d6417a3337a6DA83A6A4FdD |
| V08 | validator | 191.102.248.170 | 0x727acf9ab18c54D7602446aC9EE7Ef57da1bFAdE |
| V09 | validator | 191.102.248.171 | 0xE48037FF55b43F8FaAbb9563F13153BFA62C13b0 |
| V10 | validator | 191.102.248.172 | 0xCb4D412EF6905D3ddEA88225Be77B25660F0F7d3 |
| V11 | validator | 191.102.248.173 | 0x2B2325Fe4087970af0D24bA1902e92345A40eE90 |
| R01 | rpc | 191.102.248.174 | 0x08BF7b8F9A74Dd62177DDb89451A9126B117c0b0 |
| R02 | rpc | 191.102.248.175 | 0xc5e6895a7446292730569b1B21D95FE7ec611dFD |

## Accounts

| Name | Address | Balance |
|---|---|---|
| faucet | 0x5FA808aD3C53c8b7554bcc033F0e10c2b039bBD4 | 10000000 |
| deployer | 0xaee0b63ad1e319349cCB72b8c03b0cE522B38F16 | 1000000 |
| treasury | 0x8608620896DC2A07126de38963F8dA30Dc7B5B35 | 100000000 |
| ops | 0xf0402B7B6751Eefd7A3b1FA3Ea12462Ffd1604D0 | 1000000 |
| tester_01 | 0x3B7be0b966AA1a789fcD3DF0EE22faec665cA038 | 100000 |
| tester_02 | 0xb6051ea7F9D07E4B6dd3af56297F8ED4DD7fb8AC | 100000 |

## Systemd template
Generated at:
- /opt/scavium/testnet/scavium-besu@.service

Install manually to:
- /etc/systemd/system/scavium-besu@.service

## Suggested start order
1. B01, B02
2. V01..V11
3. R01, R02
