dev:
	go run ./cmd/neutron_query_relayer/

clean:
	@echo "Removing relayer storage state"
	-@rm -rf ./storage

test:
	 go test ./...

build:
	go build -a -o neutron_query_relayer ./cmd/neutron_query_relayer/*.go

build-docker:
	docker build . -t neutron-org/neutron-query-relayer

generate-openapi:
	@cd ./internal/subscriber/querier ; swagger generate client -f openapi.yml
