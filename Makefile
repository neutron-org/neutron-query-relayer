# todo: proper tags

dev:
	go run ./cmd/cosmos_query_relayer/

test:
	 go test ./...

build:
	go build -a -o cosmos_query_relayer ./cmd/cosmos_query_relayer/*.go

build-docker:
	docker build . -t neutron-org/cosmos-query-relayer --ssh default