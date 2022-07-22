# todo: proper tags

dev:
	go run ./cmd/cosmos_query_relayer/

test:
	CONFIG_PATH="configs/test.yml" go test ./...

build:
	go build -a -o cosmos_query_relayer ./cmd/cosmos_query_relayer/*.go

build-docker:
	go mod tidy -compat=1.17
	go mod vendor
	docker build . -t neutron-org/cosmos-query-relayer