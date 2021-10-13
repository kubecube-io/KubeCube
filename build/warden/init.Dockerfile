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

FROM alpine:3.13.4

COPY hack/install_hotplug.sh install_hotplug.sh

ENV DOWNLOAD_CHARTS true

RUN wget https://kubecube.nos-eastchina1.126.net/helm-chart/third/third-charts.tar.gz -O third-charts.tar.gz

RUN chmod +x install_hotplug.sh

CMD ["/bin/sh","install_hotplug.sh"]
