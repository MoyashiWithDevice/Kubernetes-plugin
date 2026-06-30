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