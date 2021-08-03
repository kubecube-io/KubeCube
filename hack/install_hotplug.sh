#!/bin/bash

wget https://kubecube.nos-eastchina1.126.net/helm-chart/third/third-charts.tar.gz -O third-charts.tar.gz

tar -xzvf third-charts.tar.gz

cp -r third-charts/. /root/helmchartpkg

echo "helm charts pkg downloads completed!"