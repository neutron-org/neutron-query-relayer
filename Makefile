dev:
	CONFIG_PATH="configs/dev.example.yml" go run ./cmd/cosmos_query_relayer/

test:
	CONFIG_PATH="configs/test.yml" go test ./...

build:
	go build -a -o cosmos_query_relayer ./cmd/cosmos_query_relayer/*.go
