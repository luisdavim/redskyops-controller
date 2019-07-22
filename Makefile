
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
SETUPTOOLS_IMG ?= setuptools:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Collect version information
ifdef VERSION
    LDFLAGS += -X github.com/redskyops/k8s-experiment/pkg/version.Version=${VERSION}
endif
ifneq ($(origin BUILD_METADATA), undefined)
    LDFLAGS += -X github.com/redskyops/k8s-experiment/pkg/version.BuildMetadata=${BUILD_METADATA}
endif
LDFLAGS += -X github.com/redskyops/k8s-experiment/pkg/version.GitCommit=$(shell git rev-parse HEAD)
LDFLAGS += -X github.com/redskyops/k8s-experiment/pkg/controller/trial.DefaultImage=${SETUPTOOLS_IMG}

all: manager tool

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -ldflags '$(LDFLAGS)' -o bin/manager cmd/manager/main.go

# Build tool binary for all supported platforms
tool: generate fmt vet
	GOOS=darwin GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/redskyctl-darwin-amd64 cmd/redskyctl/main.go
	GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/redskyctl-linux-amd64 cmd/redskyctl/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crd/bases

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crd/bases
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./pkg/apis/...;./pkg/controller/...;./pkg/webhook/...;./cmd/..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./pkg/apis/...;./pkg/controller/...;./pkg/webhook/...;./cmd/..."

# Build the docker images
docker-build:
	docker build . -t ${IMG} --build-arg LDFLAGS='$(LDFLAGS)'
	docker build config -t ${SETUPTOOLS_IMG} --build-arg IMG='$(IMG)'

# Push the docker images
docker-push:
	docker push ${IMG}
	docker push ${SETUPTOOLS_IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.0-beta.4
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# Generate client code
generate-client:
	client-gen --clientset-name kubernetes --input-base "" --input github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1 --output-package github.com/redskyops/k8s-experiment/pkg --go-header-file hack/boilerplate.go.txt
