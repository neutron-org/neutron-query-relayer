VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')

ldflags = -X github.com/neutron-org/neutron-query-relayer/internal/app.Version=$(VERSION) \
		  -X github.com/neutron-org/neutron-query-relayer/internal/app.Commit=$(COMMIT)

RELAYER_IMAGE_NAME ?= neutron-org/neutron-query-relayer

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

build-docker:
	docker build --build-arg LDFLAGS='$(ldflags)' . -t $(RELAYER_IMAGE_NAME)

generate-openapi:
	@cd ./internal/subscriber/querier ; swagger generate client -f openapi.yml

install:
	go install -ldflags '$(ldflags)' -a ./cmd/neutron_query_relayer
