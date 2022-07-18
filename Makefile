dev:
	CONFIG_PATH="configs/dev.yml" go run ./cmd/cosmos_query_relayer/

test:
	CONFIG_PATH="configs/test.yml" go test ./...

build:
	go build -a -o cosmos_query_relayer ./cmd/cosmos_query_relayer/*.go

build-docker:
	go mod tidy
	go mod vendor
	docker build . --build-arg some_variable_name="configs/dev.example.2-lido-chains.yml"