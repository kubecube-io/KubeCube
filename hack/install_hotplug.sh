#!/bin/bash

wget https://gitee.com/kubecube/manifests/repository/archive/master.zip

unzip master.zip

cp -r manifests/third-charts/. /root/helmchartpkg

echo "helm charts pkg downloads completed!"