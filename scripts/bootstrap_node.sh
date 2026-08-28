#!/bin/bash
set -e

echo "=== 1. Setting up 2GB Swap ==="
if [ ! -f /swapfile ]; then
  fallocate -l 2G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  echo "/swapfile none swap sw 0 0" >> /etc/fstab
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
curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.30.2+k3s1 INSTALL_K3S_EXEC="server --write-kubeconfig-mode=644" sh -
mkdir -p /home/ubuntu/.kube
cp /etc/rancher/k3s/k3s.yaml /home/ubuntu/.kube/config
chown -R ubuntu:ubuntu /home/ubuntu/.kube
echo "export KUBECONFIG=/home/ubuntu/.kube/config" >> /home/ubuntu/.bashrc

echo "=== 4. Verifying K3s ==="
sleep 5
k3s kubectl get nodes
echo "=== Bootstrap Complete ==="
