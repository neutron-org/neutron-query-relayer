FROM golang:1.17-buster as builder

RUN apt update && apt -y install openssh-server git 
RUN mkdir /app
WORKDIR /app
COPY . .
ENV GOPRIVATE=github.com/neutron-org/neutron
RUN  --mount=type=ssh mkdir -p ~/.ssh && chmod 600 ~/.ssh && \
    ssh-keyscan -H github.com >> ~/.ssh/known_hosts && \
    git config --global url."git@github.com:".insteadOf "https://github.com/" && \
    go mod download && \
    go build -a -o /go/bin/cosmos_query_relayer ./cmd/cosmos_query_relayer

FROM debian:buster
ENV RELAYER_NEUTRON_CHAIN_ALLOW_KV_CALLBACKS=1
ENV RELAYER_ALLOW_TX_QUERIES=1
RUN apt update && apt install ca-certificates curl -y && apt-get clean
ADD ["https://github.com/CosmWasm/wasmvm/raw/v1.0.0/api/libwasmvm.x86_64.so", "https://github.com/CosmWasm/wasmvm/raw/v1.0.0/api/libwasmvm.aarch64.so", "/lib/"]
ADD run.sh .
COPY --from=builder /go/bin/cosmos_query_relayer /bin/
EXPOSE 9999

ENTRYPOINT ["cosmos_query_relayer"]