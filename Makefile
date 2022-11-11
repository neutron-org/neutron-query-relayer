VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')
BUILDDIR ?= $(CURDIR)/build
DOCKER := $(shell which docker)
GO_VERSION=1.18


ldflags = -X github.com/neutron-org/neutron-query-relayer/internal/app.Version=$(VERSION) \
		  -X github.com/neutron-org/neutron-query-relayer/internal/app.Commit=$(COMMIT)

dev: clean
	go run ./cmd/neutron_query_relayer/ start

clean:
	@echo "Removing relayer storage state"
	-@rm -rf ./storage

test:
	 go test ./...

.PHONY: build
build:
	go build -ldflags '$(ldflags)' -a -o build/neutron_query_relayer ./cmd/neutron_query_relayer/*.go

build-static-linux-amd64: go.sum $(BUILDDIR)/
	$(DOCKER) buildx create --name neutronqueryrelayerbuilder || true
	$(DOCKER) buildx use neutronqueryrelayerbuilder
	$(DOCKER) buildx build \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg GIT_VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(COMMIT) \
		--platform linux/amd64 \
		-t neutronqueryrelayer-amd64 \
		--load \
		-f Dockerfile.builder .
	$(DOCKER) rm -f neutronqueryrelayerbinary || true
	$(DOCKER) create -ti --name neutronqueryrelayerbinary neutronqueryrelayer-amd64
	$(DOCKER) cp neutronqueryrelayerbinary:/bin/neutron_query_relayer $(BUILDDIR)/neutron_query_relayer-linux-amd64
	$(DOCKER) rm -f neutronqueryrelayerbinary

build-docker:
	$(DOCKER) build --build-arg LDFLAGS='$(ldflags)' . -t neutron-org/neutron-query-relayer

generate-openapi:
	@cd ./internal/subscriber/querier ; swagger generate client -f openapi.yml

install:
	go install -ldflags '$(ldflags)' -a ./cmd/neutron_query_relayer
