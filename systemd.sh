#!/bin/bash

sudo cp /opt/scavium/testnet/scavium-besu@.service /etc/systemd/system/
sudo systemctl daemon-reload
