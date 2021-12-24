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

# Build the manager binary
FROM golang:1.16 as builder

WORKDIR /workspace

# Copy the go source
COPY go.mod go.mod
COPY go.sum go.sum
COPY cmd/ cmd/
COPY config/ config/
COPY pkg/ pkg/
COPY docs/ docs/
COPY vendor/ vendor/
COPY Makefile Makefile
COPY i18n/ i18n/

# Build
RUN make build-cube

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:3.13.4
WORKDIR /
COPY --from=builder /workspace/cube .
COPY --from=builder /workspace/i18n i18n

ENTRYPOINT ["/cube"]
