VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')

ldflags = -X github.com/neutron-org/neutron-query-relayer/internal/app.Version=$(VERSION) \
		  -X github.com/neutron-org/neutron-query-relayer/internal/app.Commit=$(COMMIT) \

dev: clean
	go run ./cmd/neutron_query_relayer/

clean:
	@echo "Removing relayer storage state"
	-@rm -rf ./storage

test:
	 go test ./...

build:
	go build -ldflags '$(ldflags)' -a -o neutron_query_relayer ./cmd/neutron_query_relayer/*.go

build-docker:
	docker build . -t neutron-org/neutron-query-relayer

generate-openapi:
	@cd ./internal/subscriber/querier ; swagger generate client -f openapi.yml
