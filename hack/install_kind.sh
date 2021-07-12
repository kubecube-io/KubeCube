#!/usr/bin/env bash

curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl
chmod +x kubectl && mv kubectl /usr/local/bin/kubectl
wget https://github.com/kubernetes-sigs/kind/releases/download/v0.5.0/kind-linux-amd64 && chmod +x kind-linux-amd64 && mv kind-linux-amd64 /usr/local/bin/kind

kind create cluster --config=/etc/cube/kind/config.yaml
kubectl wait --for=condition=Ready pods --all --namespace kube-system
kubectl cluster-info