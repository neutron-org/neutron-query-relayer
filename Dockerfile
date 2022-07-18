FROM golang:1.17

ARG config_path

# these ports are just copied from dev example config
EXPOSE 16657 26657 8080

RUN mkdir /app
ADD . /app
WORKDIR /app

RUN go build -a -o cosmos_query_relayer ./cmd/cosmos_query_relayer/*.go
CMD CONFIG_PATH=$config_path go run ./cmd/cosmos_query_relayer/
