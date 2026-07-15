BINARY_NAME=kubectl-detective
KIND_CLUSTER_NAME=detective
# Make の環境に GOPATH が無いと $(GOPATH)/bin が /bin になり install が失敗する
GOPATH ?= $(shell go env GOPATH)
INSTALL_DIR ?= $(GOPATH)/bin

.PHONY: build
build:
	go build -o bin/$(BINARY_NAME) .

.PHONY: run
run:
	go run .

.PHONY: test
test:
	go test ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: clean
clean:
	rm -rf bin

.PHONY: lint
lint:
	golangci-lint run

.PHONY: all
all: fmt vet test build

BPF_DIR=bpf

.PHONY: bpf
bpf:
	docker run --rm \
		-v $(CURDIR)/$(BPF_DIR):/$(BPF_DIR) \
		-w /$(BPF_DIR) \
		gcc:latest \
		bash -c "apt-get update -qq && apt-get install -y -qq clang llvm libbpf-dev > /dev/null 2>&1 && \
		clang -O2 -g -target bpf -D__TARGET_ARCH_x86 \
			-I/usr/include \
			-c /$(BPF_DIR)/tcp_connect.bpf.c -o /$(BPF_DIR)/tcp_connect.bpf.o && \
		llvm-strip -g /$(BPF_DIR)/tcp_connect.bpf.o"
	cp $(BPF_DIR)/tcp_connect.bpf.o internal/flow/tcp_connect.bpf.o

.PHONY: vmlinux
vmlinux:
	docker run --rm \
		-v /sys/kernel/btf:/sys/kernel/btf:ro \
		-v $(PWD)/$(BPF_DIR):/$(BPF_DIR) \
		--pid=host --privileged \
		ubuntu:latest \
		bash -c "apt-get update -qq && apt-get install -y -qq bpftool > /dev/null 2>&1 && \
		bpftool btf dump file /sys/kernel/btf/vmlinux format c > /$(BPF_DIR)/vmlinux.h"

.PHONY: install
install: build
	mkdir -p $(INSTALL_DIR)
	cp bin/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY_NAME)"
	@echo "Ensure $(INSTALL_DIR) is on your PATH so 'kubectl detective' works."

.PHONY: kind-up
kind-up:
	kind create cluster \
		--name $(KIND_CLUSTER_NAME) \
		--config hack/kind-config.yaml

.PHONY: kind-down
kind-down:
	kind delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: kind-recreate
kind-recreate: kind-down kind-up

.PHONY: kind-status
kind-status:
	kubectl cluster-info --context kind-$(KIND_CLUSTER_NAME)

.PHONY: kind-apply
kind-apply:
	kubectl apply -f deploy/test-deployment.yaml

.PHONY: kind-flows
kind-flows:
	$(MAKE) install
	kubectl detective flows

.PHONY: proto
proto:
	protoc \
		--proto_path=api/proto \
		--go_out=api/detective/v1 --go_opt=paths=source_relative \
		--go-grpc_out=api/detective/v1 --go-grpc_opt=paths=source_relative \
		api/proto/detective.proto

DOCKER_IMAGE ?= kubectl-detective:latest
AGGREGATOR_NAMESPACE ?= detective

.PHONY: docker-build
docker-build:
	docker build -t $(DOCKER_IMAGE) .

.PHONY: deploy-aggregator
deploy-aggregator:
	kubectl apply -f deploy/aggregator.yaml

.PHONY: deploy-agent
deploy-agent:
	kubectl apply -f deploy/agent.yaml

.PHONY: deploy
deploy: deploy-aggregator deploy-agent

.PHONY: undeploy
undeploy:
	kubectl delete -f deploy/agent.yaml --ignore-not-found
	kubectl delete -f deploy/aggregator.yaml --ignore-not-found

.PHONY: kind-deploy
kind-deploy: build
	$(MAKE) kind-apply
	$(MAKE) deploy

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build          - build binary"
	@echo "  install        - build and install to $(INSTALL_DIR)"
	@echo "  run            - run application"
	@echo "  test           - run tests"
	@echo "  fmt            - format code"
	@echo "  vet            - run go vet"
	@echo "  lint           - run golangci-lint"
	@echo "  clean          - remove binaries"
	@echo "  all            - fmt + vet + test + build"
	@echo "  bpf            - compile eBPF object"
	@echo "  proto          - generate protobuf/gRPC code"
	@echo "  docker-build   - build Docker image"
	@echo "  deploy         - deploy agent + aggregator to cluster"
	@echo "  deploy-aggregator - deploy aggregator only"
	@echo "  deploy-agent   - deploy agent DaemonSet only"
	@echo "  undeploy       - remove detective from cluster"
	@echo "  kind-up        - create kind cluster"
	@echo "  kind-down      - delete kind cluster"
	@echo "  kind-apply     - apply test deployment"
	@echo "  kind-status    - show kind cluster info"
	@echo "  kind-deploy    - build + deploy to kind cluster"