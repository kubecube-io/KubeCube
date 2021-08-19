/*
Copyright 2021 KubeCube Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import "html/template"

const (
	scriptTemplateText = `#!/usr/bin/env bash

mkdir -p /etc/kubecube
cd /etc/kubecube

if [ -e "./manifests" ]; then
  echo -e "$(date +'%Y-%m-%d %H:%M:%S') \033[32mINFO\033[0m manifests already exist"
else
  echo -e "$(date +'%Y-%m-%d %H:%M:%S') \033[32mINFO\033[0m downloading manifests for kubecube"
  wget https://kubecube.nos-eastchina1.126.net/kubecube-installer/{{ .InstallerVersion }}/manifests.tar.gz -O manifests.tar.gz

  tar -xzvf manifests.tar.gz > /dev/null
fi

function prev_confirm() {
  echo -e "\033[32m================================================================================================\033[0m"
  echo -e "\033[32m IMPORTANT !!!                                                                                  \033[0m"
  echo -e "\033[32m You must change the args of k8s api-server before installing kubecube, steps below:            \033[0m"
  echo -e "\033[32m 1. find the manifests folder contains kube-apiserver.yaml                                      \033[0m"
  echo -e "\033[32m    generally in /etc/kubernetes/manifests of master node.                                      \033[0m"
  echo -e "\033[32m 2. add patches as below:                                                                       \033[0m"
  echo -e "\033[32m================================================================================================\033[0m"
  echo -e "\033[32m spec:                                                                                          \033[0m"
  echo -e "\033[32m   containers:                                                                                  \033[0m"
  echo -e "\033[32m     - command:                                                                                 \033[0m"
  echo -e "\033[32m         - kube-apiserver                                                                       \033[0m"
  echo -e "\033[32m         - --audit-webhook-config-file=/etc/cube/audit/audit-webhook.config                     \033[0m"
  echo -e "\033[32m         - --audit-policy-file=/etc/cube/audit/audit-policy.yaml                                \033[0m"
  echo -e "\033[32m         - --authentication-token-webhook-config-file=/etc/cube/warden/webhook.config           \033[0m"
  echo -e "\033[32m         - --audit-log-format=json                                                              \033[0m"
  echo -e "\033[32m         - --audit-log-maxage=10                                                                \033[0m"
  echo -e "\033[32m         - --audit-log-maxbackup=10                                                             \033[0m"
  echo -e "\033[32m         - --audit-log-maxsize=100                                                              \033[0m"
  echo -e "\033[32m         - --audit-log-path=/var/log/audit                                                      \033[0m"
  echo -e "\033[32m       volumeMounts:                                                                            \033[0m"
  echo -e "\033[32m       - mountPath: /var/log/audit                                                              \033[0m"
  echo -e "\033[32m         name: audit-log                                                                        \033[0m"
  echo -e "\033[32m       - mountPath: /etc/cube                                                                   \033[0m"
  echo -e "\033[32m         name: cube                                                                             \033[0m"
  echo -e "\033[32m         readOnly: true                                                                         \033[0m"
  echo -e "\033[32m   volumes:                                                                                     \033[0m"
  echo -e "\033[32m     - hostPath:                                                                                 \033[0m"
  echo -e "\033[32m         path: /var/log/audit                                                                   \033[0m"
  echo -e "\033[32m         type: DirectoryOrCreate                                                                \033[0m"
  echo -e "\033[32m       name: audit-log                                                                          \033[0m"
  echo -e "\033[32m     - hostPath:                                                                                 \033[0m"
  echo -e "\033[32m         path: /etc/cube                                                                        \033[0m"
  echo -e "\033[32m         type: DirectoryOrCreate                                                                \033[0m"
  echo -e "\033[32m       name: cube                                                                               \033[0m"
  echo -e "\033[32m================================================================================================\033[0m"
  echo -e "\033[32m Please enter 'exit' to modify args of k8s api-server \033[0m"
  echo -e "\033[32m After modify is done, please redo script and enter 'confirm' to continue \033[0m"
  while read confirm
  do
    if [[ ${confirm} = "confirm" ]]; then
      break
    elif [[ ${confirm} = "exit" ]]; then
      exit 1
    else
      continue
    fi
  done
}

function install_dependence() {
  echo -e "\033[32m================================================\033[0m"
  echo -e "\033[32m deploy hnc-manager, and wait for ready...\033[0m"
  kubectl apply -f manifests/hnc/hnc.yaml
  kubectl wait --for=condition=Ready --timeout=300s pods --all --namespace hnc-system
  sleep 7 > /dev/null

  echo -e "\033[32m================================================\033[0m"
  echo -e "\033[32m deploy local-path-storage...\033[0m"
  kubectl apply -f manifests/local-path-storage/local-path-storage.yaml

  echo -e "\033[32m================================================\033[0m"
  echo -e "\033[32m deploy metrics-server...\033[0m"
  kubectl apply -f manifests/metrics-server/metrics-server.yaml

  echo -e "\033[32m================================================\033[0m"
  echo -e "\033[32m deploy nginx ingress controller...\033[0m"
  kubectl apply -f manifests/ingress-controller/ingress-controller.yaml
}

prev_confirm

echo -e "\033[32m================================================\033[0m"
echo -e "\033[32m wait for api-server restart...\033[0m"
kubectl wait --for=condition=Ready --timeout=300s pods --all --namespace kube-system

if [ $(kubectl get nodes | wc -l) -eq 2 ]
then
  echo -e "\033[32m================================================\033[0m"
  echo -e "\033[32m delete taint of master node...\033[0m"
  kubectl get nodes | grep -v "NAME" | awk '{print $1}' | sed -n '1p' | xargs -t -i kubectl taint node {} node-role.kubernetes.io/master- > /dev/null
fi

install_dependence

curl -k -H "Content-type: application/json" -X POST https://{{ .KubeCubeHost }}:30443/api/v1/cube/clusters/register -d '{"apiVersion":"cluster.kubecube.io/v1","kind":"Cluster","metadata":{"name":"{{ .ClusterName }}"},"spec":{"kubernetesAPIEndpoint":"{{ .K8sEndpoint }}","networkType":"{{ .NetworkType }}","isMemberCluster":true,"description":"{{ .Description }}","kubeconfig":"{{ .KubeConfig }}"}}' > /dev/null
if [[ $? = 0 ]]; then
  echo -e "\033[32m================================================\033[0m"
  echo -e "\033[32m             add cluster success!               \033[0m"
  echo -e "\033[32m      please go to console and check out!       \033[0m"
  echo -e "\033[32m================================================\033[0m"
  exit 0
else
  echo -e "\033[32m================================================\033[0m"
  echo -e "\033[32m             add cluster failed.                \033[0m"
  echo -e "\033[32m================================================\033[0m"
  exit 1
fi
`
)

var (
	scriptTemplate = template.Must(template.New("scriptTemplate").Parse(scriptTemplateText))
)
