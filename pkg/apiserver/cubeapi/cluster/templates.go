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
	
function init_etcd_secret (){
  kubectl create namespace kubecube-monitoring --dry-run=client -o yaml | kubectl apply -f -
  kubectl create secret generic etcd-certs -n kubecube-monitoring --dry-run=client -o yaml \
  --from-file=ca.crt=/etc/kubernetes/pki/ca.crt \
  --from-file=client.crt=/etc/kubernetes/pki/apiserver-etcd-client.crt \
  --from-file=client.key=/etc/kubernetes/pki/apiserver-etcd-client.key | kubectl apply -f -
}	

function install_dependence() {
  echo -e "\033[32m================================================\033[0m"
  echo -e "\033[32m deploy hnc-manager, and wait for ready...\033[0m"
  kubectl apply -f manifests/hnc/hnc.yaml

  hnc_ready="0/2"
  while [ ${hnc_ready} != "2/2" ]
  do
    sleep 5 > /dev/null
    hnc_ready=$(kubectl get pod -n hnc-system | awk '{print $2}' | sed -n '2p')
  done
  sleep 20 > /dev/null

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

if [ $(kubectl get nodes | wc -l) -eq 2 ]
then
  echo -e "\033[32m================================================\033[0m"
  echo -e "\033[32m delete taint of master node...\033[0m"
  kubectl get nodes | grep -v "NAME" | awk '{print $1}' | sed -n '1p' | xargs -t -i kubectl taint node {} node-role.kubernetes.io/master- > /dev/null
fi

install_dependence
init_etcd_secret

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
