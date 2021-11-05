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

set -e

# default download_url
download_url="https://kubecube.nos-eastchina1.126.net/helm-chart/third/third-charts.tar.gz"

if [ ${DOWNLOAD_URL} ];then
  if [ ${DOWNLOAD_URL} != "" ]; then
      download_url=${DOWNLOAD_URL}
  fi
fi

if [ ${DOWNLOAD_CHARTS} = "true" ]; then
  # use remote helm charts to replace of local charts pkg
  echo "download charts pkg form remote: ${download_url}"
  wget ${download_url} -O third-charts.tar.gz
else
  echo "use local charts pkg"
fi

tar -xzvf third-charts.tar.gz

cp -r third-charts/. /root/helmchartpkg

echo "helm charts pkg build up completed!"