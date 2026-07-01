BINARY_NAME=kubectl-detective
KIND_CLUSTER_NAME=detective


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
		-v $(PWD)/$(BPF_DIR):/$(BPF_DIR) \
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
	cp bin/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)

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

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build  - build binary"
	@echo "  run    - run application"
	@echo "  test   - run tests"
	@echo "  fmt    - format code"
	@echo "  vet    - run go vet"
	@echo "  lint   - run golangci-lint"
	@echo "  clean  - remove binaries"
	@echo "  all    - fmt + vet + test + build"