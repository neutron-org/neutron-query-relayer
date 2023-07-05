FROM golang:1.20-buster as builder

ARG LDFLAGS
RUN mkdir /app
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags "${LDFLAGS}" -a -o build/neutron_query_relayer ./cmd/neutron_query_relayer/*.go

FROM debian:buster
RUN apt update && apt install ca-certificates curl -y && apt-get clean
ADD ["https://github.com/CosmWasm/wasmvm/releases/download/v1.2.4/libwasmvm.x86_64.so","https://github.com/CosmWasm/wasmvm/releases/download/v1.2.4/libwasmvm.aarch64.so","/lib/"]
ADD run.sh .
COPY --from=builder /app/build/neutron_query_relayer /bin/
EXPOSE 9999

ENTRYPOINT ["neutron_query_relayer", "start"]
