# syntax = docker/dockerfile:1.0-experimental
FROM golang:1.17-buster as builder

RUN apt update
RUN apt -y install openssh-server git

RUN mkdir /app
WORKDIR /app

RUN mkdir -p ~/.ssh && chmod 600 ~/.ssh
RUN ssh-keyscan -H github.com >> ~/.ssh/known_hosts
ENV GOPRIVATE=github.com/neutron-org/neutron
RUN git config --global url."git@github.com:".insteadOf "https://github.com/"

COPY . .
RUN --mount=type=ssh go mod download
RUN go build -a -o /go/bin/cosmos_query_relayer ./cmd/cosmos_query_relayer


FROM debian:buster
RUN apt update && apt install ca-certificates -y
ADD https://github.com/CosmWasm/wasmvm/raw/0ff9c3a666ef15b12e447e830cc32a3314325ef0/api/libwasmvm.x86_64.so /lib/libwasmvm.x86_64.so
COPY --from=builder /go/bin/cosmos_query_relayer /bin/
EXPOSE 9999

ENTRYPOINT ["cosmos_query_relayer"]