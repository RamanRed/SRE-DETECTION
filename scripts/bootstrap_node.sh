#!/usr/bin/env bash
set -Eeuo pipefail

echo "=== 1. Setting up 2GB swap for K3s operational headroom ==="
if [ ! -f /swapfile ]; then
  fallocate -l 2G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile
fi
if ! swapon --show=NAME --noheadings | grep -qx '/swapfile'; then
  swapon /swapfile
fi
if ! grep -q '^/swapfile[[:space:]]' /etc/fstab; then
  echo "/swapfile none swap sw 0 0" >> /etc/fstab
fi
if [[ ! -f /sys/fs/cgroup/memory.swap.max ]]; then
  echo "cgroup v2 memory/swap accounting is required" >&2
  exit 1
fi
free -h

echo "=== 2. Installing Docker Engine ==="
apt-get update -qq
apt-get install -y -qq apt-transport-https ca-certificates curl gnupg lsb-release git
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg --yes
chmod a+r /etc/apt/keyrings/docker.gpg
UBUNTU_CODENAME=$(lsb_release -cs)
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${UBUNTU_CODENAME} stable" > /etc/apt/sources.list.d/docker.list
apt-get update -qq
apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
usermod -aG docker ubuntu
systemctl enable docker && systemctl start docker
docker --version

echo "=== 3. Installing K3s Kubernetes ==="
curl -sfL https://get.k3s.io | \
  INSTALL_K3S_VERSION=v1.36.2+k3s1 \
  INSTALL_K3S_EXEC="server --kubelet-arg=fail-swap-on=false" \
  sh -
chmod 600 /etc/rancher/k3s/k3s.yaml
install -d -m 0700 -o ubuntu -g ubuntu /home/ubuntu/.kube
install -m 0600 -o ubuntu -g ubuntu /etc/rancher/k3s/k3s.yaml /home/ubuntu/.kube/config
if ! grep -q '^export KUBECONFIG=/home/ubuntu/.kube/config$' /home/ubuntu/.bashrc; then
  echo "export KUBECONFIG=/home/ubuntu/.kube/config" >> /home/ubuntu/.bashrc
fi

echo "=== 4. Verifying K3s ==="
sleep 5
k3s kubectl get nodes
echo "=== Bootstrap Complete ==="
