#!/bin/bash

sudo groupadd --system scavium
sudo useradd --system --gid scavium --home-dir /opt/scavium --shell /usr/sbin/nologin scavium
sudo chown -R scavium:scavium /opt/scavium
