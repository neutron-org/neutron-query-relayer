package subscriber_test

import (
	"context"
	"fmt"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
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

func TestSubscribeContinuesOnMissingQuery(t *testing.T) {
	// Create a new controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfgLogger := zap.NewProductionConfig()
	logger, err := cfgLogger.Build()
	require.NoError(t, err)

	rpcClient := mock_subscriber.NewMockRpcHttpClient(ctrl)
	restQuery := mock_subscriber.NewMockRestHttpQuery(ctrl)

	updateEvents := make(chan ctypes.ResultEvent)

	rpcClient.EXPECT().Start()
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).Return(updateEvents, nil)
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any())

	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())

	ctx, cancel := context.WithCancel(context.Background())

	// expect to return not found error on queryId = "1"
	queryId := "1"
	restQuery.EXPECT().NeutronInterchainQueriesRegisteredQuery(&query.NeutronInterchainQueriesRegisteredQueryParams{
		QueryID: &queryId,
		Context: ctx,
	}).Return(nil, fmt.Errorf("not found"))

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
		Registry:     registry.New(&registry.RegistryConfig{Addresses: []string{"owner"}}),
	}
	s, err := subscriber.NewSubscriber(&cfg, rpcClient, restQuery, logger)
	assert.NoError(t, err)

	go func() {
		// send the update event that we'll return non-existent query
		events := make(map[string][]string)
		// to pass the checkEvents func
		events[subscriber.ConnectionIdAttr] = []string{"kek"}
		events[subscriber.KvKeyAttr] = []string{"kek"}
		events[subscriber.TransactionsFilterAttr] = []string{"kek"}
		events[subscriber.QueryIdAttr] = []string{"1"}
		events[subscriber.TypeAttr] = []string{"kek"}

		// to pass owner check
		events[subscriber.OwnerAttr] = []string{"owner"}

		updateEvents <- ctypes.ResultEvent{
			Query:  "",
			Data:   nil,
			Events: events,
		}

		// should terminate Subscribe() function without error
		cancel()
	}()

	// as we have `expect` for `NeutronInterchainQueriesRegisteredQueries`,
	// we are sure that processUpdateEvent was executed before the context `cancel()` call
	err = s.Subscribe(ctx, queriesTasksQueue)
	assert.Equal(t, err, nil)
}
