#!/usr/bin/env bash

#Copyright 2021 KubeCube Authors
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl
chmod +x kubectl && mv kubectl /usr/local/bin/kubectl
wget https://github.com/kubernetes-sigs/kind/releases/download/v0.5.0/kind-linux-amd64 && chmod +x kind-linux-amd64 && mv kind-linux-amd64 /usr/local/bin/kind

kind create cluster --config=/etc/cube/kind/config.yaml
kubectl wait --for=condition=Ready pods --all --namespace kube-system
kubectl cluster-info