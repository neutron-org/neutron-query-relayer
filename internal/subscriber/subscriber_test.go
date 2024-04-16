package subscriber_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client/query"
	mock_subscriber "github.com/neutron-org/neutron-query-relayer/testutil/mocks/subscriber"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
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

func TestSubscribeWithEmptyQueryIDs(t *testing.T) {
	// Create a new controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfgLogger := zap.NewProductionConfig()
	logger, err := cfgLogger.Build()
	require.NoError(t, err)

	rpcClient := mock_subscriber.NewMockRpcHttpClient(ctrl)
	restQuery := mock_subscriber.NewMockRestHttpQuery(ctrl)

	updateEvents := make(chan ctypes.ResultEvent)
	blockEvents := make(chan ctypes.ResultEvent)
	rpcClient.EXPECT().Start()
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).Return(updateEvents, nil)
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).Return(blockEvents, nil)

	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())

	restQuery.EXPECT().NeutronInterchainQueriesRegisteredQueries(gomock.Any()).Return(&query.NeutronInterchainQueriesRegisteredQueriesOK{
		Payload: &query.NeutronInterchainQueriesRegisteredQueriesOKBody{
			Pagination: &query.NeutronInterchainQueriesRegisteredQueriesOKBodyPagination{
				NextKey: nil,
				Total:   "",
			},
			RegisteredQueries: []*query.NeutronInterchainQueriesRegisteredQueriesOKBodyRegisteredQueriesItems0{
				{
					ID:                             "1",
					Owner:                          "owner",
					QueryType:                      "kv",
					UpdatePeriod:                   "1",
					LastSubmittedResultLocalHeight: "0",
					LastSubmittedResultRemoteHeight: &query.NeutronInterchainQueriesRegisteredQueriesOKBodyRegisteredQueriesItems0LastSubmittedResultRemoteHeight{
						RevisionHeight: "0",
						RevisionNumber: "0",
					},
				},
				{
					ID:                             "2",
					Owner:                          "owner",
					QueryType:                      "kv",
					UpdatePeriod:                   "1",
					LastSubmittedResultLocalHeight: "0",
					LastSubmittedResultRemoteHeight: &query.NeutronInterchainQueriesRegisteredQueriesOKBodyRegisteredQueriesItems0LastSubmittedResultRemoteHeight{
						RevisionHeight: "0",
						RevisionNumber: "0",
					},
				},
			},
		},
	}, nil)

	queriesTasksQueue := make(chan neutrontypes.RegisteredQuery, 100)
	cfg := subscriber.Config{
		ConnectionID: "",
		WatchedTypes: []neutrontypes.InterchainQueryType{
			"kv",
			"tx",
		},
		Registry: registry.New(&registry.RegistryConfig{
			Addresses: make([]string, 0),
			QueryIDs:  make([]uint64, 0), // do not filter by query ID
		}),
	}
	s, err := subscriber.NewSubscriber(&cfg, rpcClient, restQuery, logger)
	assert.NoError(t, err)

	generateNewBlock := func() func() {
		height := int64(1)
		return func() {
			rpcClient.EXPECT().Status(gomock.Any()).Return(&ctypes.ResultStatus{
				SyncInfo: ctypes.SyncInfo{
					LatestBlockHeight: height,
				},
			}, nil)

			blockEvents <- ctypes.ResultEvent{}
			height++
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		{
			// in this block we are going to check queriesTasksQueue are exactly 1 and 2
			generateNewBlock()
			queries := []neutrontypes.RegisteredQuery{
				<-queriesTasksQueue,
				<-queriesTasksQueue,
			}
			// reorder the queries by id, as Subscriber.activeQueries is a map, it does not guarantee the order of the iteration
			sort.Slice(queries, func(i, j int) bool {
				return queries[i].Id < queries[j].Id
			})
			assert.Equal(t, uint64(1), queries[0].Id)
			assert.Equal(t, uint64(2), queries[1].Id)
			assert.Equal(t, 0, len(queriesTasksQueue))
		}

		{
			// in this block we are going to confirm that query id 3 is added to activeQueries
			queryId := "3"
			restQuery.EXPECT().NeutronInterchainQueriesRegisteredQuery(&query.NeutronInterchainQueriesRegisteredQueryParams{
				QueryID: &queryId,
				Context: ctx,
			}).Return(&query.NeutronInterchainQueriesRegisteredQueryOK{
				Payload: &query.NeutronInterchainQueriesRegisteredQueryOKBody{
					RegisteredQuery: &query.NeutronInterchainQueriesRegisteredQueryOKBodyRegisteredQuery{
						ID:                             queryId,
						Owner:                          "owner",
						QueryType:                      "kv",
						UpdatePeriod:                   "1",
						LastSubmittedResultLocalHeight: "0",
						LastSubmittedResultRemoteHeight: &query.NeutronInterchainQueriesRegisteredQueryOKBodyRegisteredQueryLastSubmittedResultRemoteHeight{
							RevisionHeight: "0",
							RevisionNumber: "0",
						},
					},
				},
			}, nil)
			events := make(map[string][]string)
			events[subscriber.QueryIdAttr] = []string{queryId}
			events[subscriber.ConnectionIdAttr] = []string{"kek"}
			events[subscriber.KvKeyAttr] = []string{"kek"}
			events[subscriber.TransactionsFilterAttr] = []string{"kek"}
			events[subscriber.TypeAttr] = []string{"kek"}
			events[subscriber.OwnerAttr] = []string{"owner"}

			updateEvents <- ctypes.ResultEvent{Events: events}

			generateNewBlock()
			queries := []neutrontypes.RegisteredQuery{
				<-queriesTasksQueue,
				<-queriesTasksQueue,
				<-queriesTasksQueue,
			}
			// reorder the queries by id, as Subscriber.activeQueries is a map, it does not guarantee the order of the iteration
			sort.Slice(queries, func(i, j int) bool {
				return queries[i].Id < queries[j].Id
			})
			assert.Equal(t, uint64(1), queries[0].Id)
			assert.Equal(t, uint64(2), queries[1].Id)
			assert.Equal(t, uint64(3), queries[2].Id)
			assert.Equal(t, 0, len(queriesTasksQueue))
		}

		// should terminate Subscribe() function
		cancel()
	}()

	err = s.Subscribe(ctx, queriesTasksQueue)
	assert.Equal(t, err, nil)
}

func TestSubscribeWithSpecifiedQueryIDs(t *testing.T) {
	// Create a new controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfgLogger := zap.NewProductionConfig()
	logger, err := cfgLogger.Build()
	require.NoError(t, err)

	rpcClient := mock_subscriber.NewMockRpcHttpClient(ctrl)
	restQuery := mock_subscriber.NewMockRestHttpQuery(ctrl)

	updateEvents := make(chan ctypes.ResultEvent)
	blockEvents := make(chan ctypes.ResultEvent)
	rpcClient.EXPECT().Start()
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).Return(updateEvents, nil)
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).Return(blockEvents, nil)

	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())
	rpcClient.EXPECT().Unsubscribe(gomock.Any(), gomock.Any(), gomock.Any())

	restQuery.EXPECT().NeutronInterchainQueriesRegisteredQueries(gomock.Any()).Return(&query.NeutronInterchainQueriesRegisteredQueriesOK{
		Payload: &query.NeutronInterchainQueriesRegisteredQueriesOKBody{
			Pagination: &query.NeutronInterchainQueriesRegisteredQueriesOKBodyPagination{
				NextKey: nil,
				Total:   "",
			},
			RegisteredQueries: []*query.NeutronInterchainQueriesRegisteredQueriesOKBodyRegisteredQueriesItems0{
				{
					ID:                             "1",
					Owner:                          "owner",
					QueryType:                      "kv",
					UpdatePeriod:                   "1",
					LastSubmittedResultLocalHeight: "0",
					LastSubmittedResultRemoteHeight: &query.NeutronInterchainQueriesRegisteredQueriesOKBodyRegisteredQueriesItems0LastSubmittedResultRemoteHeight{
						RevisionHeight: "0",
						RevisionNumber: "0",
					},
				},
				{
					ID:                             "2",
					Owner:                          "owner",
					QueryType:                      "kv",
					UpdatePeriod:                   "1",
					LastSubmittedResultLocalHeight: "0",
					LastSubmittedResultRemoteHeight: &query.NeutronInterchainQueriesRegisteredQueriesOKBodyRegisteredQueriesItems0LastSubmittedResultRemoteHeight{
						RevisionHeight: "0",
						RevisionNumber: "0",
					},
				},
				{
					ID:                             "3",
					Owner:                          "owner",
					QueryType:                      "tx",
					UpdatePeriod:                   "1",
					LastSubmittedResultLocalHeight: "0",
					LastSubmittedResultRemoteHeight: &query.NeutronInterchainQueriesRegisteredQueriesOKBodyRegisteredQueriesItems0LastSubmittedResultRemoteHeight{
						RevisionHeight: "0",
						RevisionNumber: "0",
					},
				},
			},
		},
	}, nil)

	queriesTasksQueue := make(chan neutrontypes.RegisteredQuery, 100)
	cfg := subscriber.Config{
		ConnectionID: "",
		WatchedTypes: []neutrontypes.InterchainQueryType{
			"kv",
			"tx",
		},
		Registry: registry.New(&registry.RegistryConfig{
			Addresses: make([]string, 0),
			QueryIDs:  []uint64{1, 3, 5}, // We only handle queries which id equals 1, 3 or 5
		}),
	}
	s, err := subscriber.NewSubscriber(&cfg, rpcClient, restQuery, logger)
	assert.NoError(t, err)

	generateNewBlock := func() func() {
		height := int64(1)
		return func() {
			rpcClient.EXPECT().Status(gomock.Any()).Return(&ctypes.ResultStatus{
				SyncInfo: ctypes.SyncInfo{
					LatestBlockHeight: height,
				},
			}, nil)

			blockEvents <- ctypes.ResultEvent{}
			height++
		}
	}()
	emitUpdateEventForQueryID := func(queryID string) {
		events := make(map[string][]string)
		events[subscriber.QueryIdAttr] = []string{queryID}
		events[subscriber.ConnectionIdAttr] = []string{"kek"}
		events[subscriber.KvKeyAttr] = []string{"kek"}
		events[subscriber.TransactionsFilterAttr] = []string{"kek"}
		events[subscriber.TypeAttr] = []string{"kek"}
		events[subscriber.OwnerAttr] = []string{"owner"}
		updateEvents <- ctypes.ResultEvent{Events: events}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		{
			// in this block we are going to check queriesTasksQueue are exactly 1 and 3
			generateNewBlock()
			queries := []neutrontypes.RegisteredQuery{
				<-queriesTasksQueue,
				<-queriesTasksQueue,
			}
			// reorder the queries by id, as Subscriber.activeQueries is a map, it does not guarantee the order of the iteration
			sort.Slice(queries, func(i, j int) bool {
				return queries[i].Id < queries[j].Id
			})
			assert.Equal(t, uint64(1), queries[0].Id)
			assert.Equal(t, uint64(3), queries[1].Id)
			assert.Equal(t, 0, len(queriesTasksQueue))
		}

		{
			// in this block we are going to confirm that query id 4 is dropped cause we have no intention to hand it
			emitUpdateEventForQueryID("4")
			generateNewBlock()
			queries := []neutrontypes.RegisteredQuery{
				<-queriesTasksQueue,
				<-queriesTasksQueue,
			}
			// reorder the queries by id, as Subscriber.activeQueries is a map, it does not guarantee the order of the iteration
			sort.Slice(queries, func(i, j int) bool {
				return queries[i].Id < queries[j].Id
			})
			assert.Equal(t, uint64(1), queries[0].Id)
			assert.Equal(t, uint64(3), queries[1].Id)
			assert.Equal(t, 0, len(queriesTasksQueue))
		}

		{
			// in this block we are going to confirm that query id 5 is added to activeQueries
			queryId := "5"
			restQuery.EXPECT().NeutronInterchainQueriesRegisteredQuery(&query.NeutronInterchainQueriesRegisteredQueryParams{
				QueryID: &queryId,
				Context: ctx,
			}).Return(&query.NeutronInterchainQueriesRegisteredQueryOK{
				Payload: &query.NeutronInterchainQueriesRegisteredQueryOKBody{
					RegisteredQuery: &query.NeutronInterchainQueriesRegisteredQueryOKBodyRegisteredQuery{
						ID:                             queryId,
						Owner:                          "owner",
						QueryType:                      "kv",
						UpdatePeriod:                   "1",
						LastSubmittedResultLocalHeight: "0",
						LastSubmittedResultRemoteHeight: &query.NeutronInterchainQueriesRegisteredQueryOKBodyRegisteredQueryLastSubmittedResultRemoteHeight{
							RevisionHeight: "0",
							RevisionNumber: "0",
						},
					},
				},
			}, nil)
			emitUpdateEventForQueryID(queryId)

			generateNewBlock()
			queries := []neutrontypes.RegisteredQuery{
				<-queriesTasksQueue,
				<-queriesTasksQueue,
				<-queriesTasksQueue,
			}
			// reorder the queries by id, as Subscriber.activeQueries is a map, it does not guarantee the order of the iteration
			sort.Slice(queries, func(i, j int) bool {
				return queries[i].Id < queries[j].Id
			})
			assert.Equal(t, uint64(1), queries[0].Id)
			assert.Equal(t, uint64(3), queries[1].Id)
			assert.Equal(t, uint64(5), queries[2].Id)
			assert.Equal(t, 0, len(queriesTasksQueue))
		}

		// should terminate Subscribe() function
		cancel()
	}()

	err = s.Subscribe(ctx, queriesTasksQueue)
	assert.Equal(t, err, nil)
}
