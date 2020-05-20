# +-------------------------------------------------------------------------
# | Copyright (C) 2018 Yunify, Inc.
# +-------------------------------------------------------------------------
# | Licensed under the Apache License, Version 2.0 (the "License");
# | you may not use this work except in compliance with the License.
# | You may obtain a copy of the License in the LICENSE file, or at:
# |
# | http://www.apache.org/licenses/LICENSE-2.0
# |
# | Unless required by applicable law or agreed to in writing, software
# | distributed under the License is distributed on an "AS IS" BASIS,
# | WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# | See the License for the specific language governing permissions and
# | limitations under the License.
# +-------------------------------------------------------------------------

.PHONY: all get-api sidecar controller webhook

all: get-api sidecar controller

SIDECAR_IMAGE_NAME=kubespheredev/storage-capability-sidecar
SIDECAR_VERSION=v0.1.0
CONTROLLER_IMAGE_NAME=kubespheredev/storage-capability-controller
CONTROLLER_VERSION=v0.1.0
WEBHOOK_IMAGE_NAME=kubespheredev/storage-capability-webhook
WEBHOOK_VERSION=v0.1.0

get-api: fmt
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -a -ldflags '-extldflags "-static"' -o  _output/get-api ./cmd/get-api/main.go

sidecar: fmt
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -a -ldflags '-extldflags "-static"' -o  _output/sidecar ./cmd/sidecar/main.go
	docker build -t ${SIDECAR_IMAGE_NAME}:${SIDECAR_VERSION} -f build/sidecar/Dockerfile .

controller: fmt
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -a -ldflags '-extldflags "-static"' -o  _output/controller ./cmd/controller/main.go
	docker build -t ${CONTROLLER_IMAGE_NAME}:${CONTROLLER_VERSION} -f build/controller/Dockerfile .

webhook: fmt
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -a -ldflags '-extldflags "-static"' -o  _output/webhook ./cmd/webhook/main.go
	docker build -t ${WEBHOOK_IMAGE_NAME}:${WEBHOOK_VERSION} -f build/webhook/Dockerfile .

fmt:
	go fmt ./cmd/... ./pkg/...