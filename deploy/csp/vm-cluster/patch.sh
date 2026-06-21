#!/usr/bin/env bash
set -euo pipefail

NODES=(172.16.0.163 172.16.0.28 172.16.0.242 172.16.0.246 172.16.0.30 172.16.0.6)
for N in "${NODES[@]}"; do
  if [ "$N" == "172.16.0.163" ]; then
    sudo apt-get update && sudo apt-get install -y conntrack socat && sudo systemctl restart containerd
  else
    ssh -o StrictHostKeyChecking=no ubuntu@$N "sudo apt-get update && sudo apt-get install -y conntrack socat && sudo systemctl restart containerd"
  fi
done
~/orchestrate_csp_deploy.sh
