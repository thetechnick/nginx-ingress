export CGO_ENABLED:=0

VERSION=$(shell ./scripts/git-version)
REPO=gitlab.thetechnick.ninja/thetechnick/nginx-ingress
LD_FLAGS="-w -X $(REPO)/pkg/version.Version=$(VERSION)"

all: build

build: clean bin/nginx-ingress

bin/%:
	@go build -o bin/$* -v -ldflags $(LD_FLAGS) $(REPO)

test:
	@./scripts/test

clean:
	@rm -rf bin

.PHONY: all build clean test
