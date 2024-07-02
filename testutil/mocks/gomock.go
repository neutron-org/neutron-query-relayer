package mocks

//go:generate mockgen -source=./../../internal/subscriber/clients.go -destination ./subscriber/expected_clients.go
//go:generate mockgen -source=$GOPATH/pkg/mod/github.com/cometbft/cometbft@v0.38.7/rpc/client/interface.go -destination ./submit/mock_client.go
