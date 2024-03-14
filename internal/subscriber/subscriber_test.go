package subscriber_test

import (
	"context"
	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client/query"
	mock_subscriber "github.com/neutron-org/neutron-query-relayer/testutil/mocks/subscriber"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestDoneShouldEndSubscribe(t *testing.T) {
	// Create a new controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfgLogger := zap.NewProductionConfig()
	logger, err := cfgLogger.Build()
	require.NoError(t, err)

	rpcClient := mock_subscriber.NewMockRpcHttpClient(ctrl)
	restQuery := mock_subscriber.NewMockRestHttpQuery(ctrl)

	rpcClient.EXPECT().Start()
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any())

	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())

	//registeredQueries := make(map[string]*neutrontypes.RegisteredQuery)
	restQuery.EXPECT().NeutronInterchainQueriesRegisteredQueries(gomock.Any()).Return(&query.NeutronInterchainQueriesRegisteredQueriesOK{
		Payload: &query.NeutronInterchainQueriesRegisteredQueriesOKBody{
			Pagination: &query.NeutronInterchainQueriesRegisteredQueriesOKBodyPagination{
				NextKey: nil,
				Total:   "",
			},
			RegisteredQueries: []*query.NeutronInterchainQueriesRegisteredQueriesOKBodyRegisteredQueriesItems0{},
		},
	}, nil)

	queriesTasksQueue := make(chan neutrontypes.RegisteredQuery, 100)
	cfg := subscriber.Config{
		ConnectionID: "",
		WatchedTypes: nil,
		Registry:     registry.New(&registry.RegistryConfig{Addresses: make([]string, 0)}),
	}
	s, err := subscriber.NewSubscriber(&cfg, rpcClient, restQuery, logger)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// should terminate Subscribe() function
		cancel()
	}()

	err = s.Subscribe(ctx, queriesTasksQueue)
	assert.Equal(t, err, nil)
}

// Test that subscribe does not end on
// processUpdateEvent -> #s.getNeutronRegisteredQuery(ctx, queryID) returning error
