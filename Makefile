dev:
	go run ./cmd/neutron_query_relayer/

test:
	 go test ./...

build:
	go build -a -o neutron_query_relayer ./cmd/neutron_query_relayer/*.go

build-docker:
	docker build . -t neutron-org/neutron-query-relayer --ssh default