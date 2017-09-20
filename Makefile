export CGO_ENABLED:=0

VERSION=$(shell ./scripts/git-version)
REPO=github.com/thetechnick/nginx-ingress
LD_FLAGS="-w -X $(REPO)/pkg/version.Version=$(VERSION)"

all: build

build: clean bin/lbc bin/agent

bin/%:
	@go build -o bin/$* -v -ldflags $(LD_FLAGS) $(REPO)/cmd/$*

container: build
	docker build -f Dockerfile.lbc -t quay.io/nico_schieder/ingress-lbc:$(VERSION) .
	docker build -f Dockerfile.agent -t quay.io/nico_schieder/ingress-agent:$(VERSION) .
	docker push quay.io/nico_schieder/ingress-lbc:$(VERSION)
	docker push quay.io/nico_schieder/ingress-agent:$(VERSION)

test:
	@TEST_FLAGS="-short" ./scripts/test

test-all:
	./scripts/test

clean:
	@rm -rf bin/nginx-ingress
	@rm -rf bin/agent

tools: bin/protoc bin/protoc-gen-go

codegen: tools
	@./scripts/codegen

bin/protoc:
	@./scripts/get-protoc

bin/protoc-gen-go:
	@go build -o bin/protoc-gen-go $(REPO)/vendor/github.com/golang/protobuf/protoc-gen-go

.PHONY: all clean codegen tools test build test-all container
