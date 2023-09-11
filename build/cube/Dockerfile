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
FROM golang:1.20.7-alpine3.18 as builder

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

# Build
RUN	CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -mod=vendor -a -o cube cmd/cube/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:3.18
WORKDIR /
COPY --from=builder /workspace/cube .

ENTRYPOINT ["/cube"]
