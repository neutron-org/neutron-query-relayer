# syntax = docker/dockerfile:1.0-experimental
FROM golang:1.17-alpine as build

RUN apk update && apk add openssh && apk add git

RUN mkdir /app
WORKDIR /app

RUN mkdir -p ~/.ssh && chmod 600 ~/.ssh
RUN ssh-keyscan -H github.com >> ~/.ssh/known_hosts
ENV GOPRIVATE=github.com/neutron-org/neutron
RUN git config --global url."git@github.com:".insteadOf "https://github.com/"

COPY . .
RUN --mount=type=ssh go mod download

RUN go build -a -o /go/bin/cosmos_query_relayer ./cmd/cosmos_query_relayer


FROM alpine:3.12
RUN apk --no-cache add ca-certificates
COPY --from=build /go/bin/cosmos_query_relayer /bin/

EXPOSE 8080

ENTRYPOINT ["cosmos_query_relayer"]