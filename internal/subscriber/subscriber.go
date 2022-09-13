package subscriber

import (
	"context"
	"fmt"
	restClient "github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client"
	"time"

	"github.com/tendermint/tendermint/rpc/client/http"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"github.com/zyedidia/generic/queue"
)

var (
	unsubscribeTimeout = time.Second * 5
	restClientBasePath = "/"
	restClientSchemes  = []string{"http"}
)

// NewSubscriber creates a new Subscriber instance ready to subscribe on the given chain's events.
func NewSubscriber(
	rpcAddress string,
	targetChainID string,
	targetConnectionID string,
	registry *registry.Registry,
	watchedTypes []neutrontypes.InterchainQueryType,
	logger *zap.Logger,
) (*Subscriber, error) {
	rpcClient, err := http.New(rpcAddress, "/websocket")
	if err != nil {
		return nil, fmt.Errorf("could not create new tendermint rpcClient: %w", err)
	}

	if err = rpcClient.Start(); err != nil {
		return nil, fmt.Errorf("could not start tendermint rpcClient: %w", err)
	}

	watchedTypesMap := make(map[neutrontypes.InterchainQueryType]struct{})
	for _, queryType := range watchedTypes {
		watchedTypesMap[queryType] = struct{}{}
	}

	return &Subscriber{
		rpcClient: rpcClient,
		restClient: restClient.NewHTTPClientWithConfig(nil, &restClient.TransportConfig{
			Host:     "127.0.0.1:1316", // TODO(oopcode): add config variable
			BasePath: restClientBasePath,
			Schemes:  restClientSchemes,
		}),

		rpcAddress:         rpcAddress,
		targetChainID:      targetChainID,
		targetConnectionID: targetConnectionID,
		registry:           registry,
		logger:             logger,
		watchedTypes:       watchedTypesMap,

		activeQueries: map[string]*neutrontypes.RegisteredQuery{},
	}, nil
}

// Subscriber is responsible for subscribing on chain's ICQ events. It parses incoming events,
// filters them in accordance with the Registry configuration and watchedTypes, and provides a
// stream of split to KV and TX messages.
type Subscriber struct {
	rpcClient  *http.HTTP                 // Used to subscribe to events
	restClient *restClient.HTTPAPIConsole // Used to run Neutron-specific queries using the REST

	rpcAddress         string
	targetChainID      string
	targetConnectionID string
	registry           *registry.Registry
	logger             *zap.Logger
	watchedTypes       map[neutrontypes.InterchainQueryType]struct{}

	activeQueries map[string]*neutrontypes.RegisteredQuery
}

// Subscribe subscribes to 3 types of events: 1. a new block was created, 2. a query was updated (created / updated),
// 3. a query was removed.
func (s *Subscriber) Subscribe(ctx context.Context, tasks *queue.Queue[neutrontypes.RegisteredQuery]) error {
	queries, err := s.getNeutronRegisteredQueries(ctx)
	if err != nil {
		return fmt.Errorf("could not getNeutronRegisteredQueries: %w", err)
	}
	s.activeQueries = queries

	updateEvents, err := s.rpcClient.Subscribe(ctx, s.subscriberName(), s.subscribeQueryUpdated())
	if err != nil {
		return fmt.Errorf("could not subscribe to events: %w", err)
	}

	removeEvents, err := s.rpcClient.Subscribe(ctx, s.subscriberName(), s.subscribeQueryRemoved())
	if err != nil {
		return fmt.Errorf("could not subscribe to events: %w", err)
	}

	blockEvents, err := s.rpcClient.Subscribe(ctx, s.subscriberName(), s.subscribeQueryBlock())
	if err != nil {
		return fmt.Errorf("could not subscribe to events: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			s.unsubscribe()
			return nil
		case <-blockEvents:
			if err := s.processBlockEvent(ctx, tasks); err != nil {
				return fmt.Errorf("failed to processblockEvent: %w", err)
			}
		case event := <-updateEvents:
			if err := s.processUpdateEvent(ctx, event); err != nil {
				return fmt.Errorf("failed to processUpdateEvent: %w", err)
			}
		case event := <-removeEvents:
			if err := s.processRemoveEvent(event); err != nil {
				return fmt.Errorf("failed to processRemoveEvent: %w", err)
			}
		}
	}
}

func (s *Subscriber) unsubscribe() {
	ctx, cancel := context.WithTimeout(context.Background(), unsubscribeTimeout)
	defer cancel()

	if err := s.rpcClient.Unsubscribe(ctx, s.subscriberName(), s.subscribeQueryUpdated()); err != nil {
		s.logger.Error("failed to Unsubscribe from tm events",
			zap.Error(err), zap.String("ActiveQuery", s.subscribeQueryUpdated()))
	}

	if err := s.rpcClient.Unsubscribe(ctx, s.subscriberName(), s.subscribeQueryRemoved()); err != nil {
		s.logger.Error("failed to Unsubscribe from tm events",
			zap.Error(err), zap.String("ActiveQuery", s.subscribeQueryRemoved()))
	}

	if err := s.rpcClient.Unsubscribe(ctx, s.subscriberName(), s.subscribeQueryBlock()); err != nil {
		s.logger.Error("failed to Unsubscribe from tm events",
			zap.Error(err), zap.String("ActiveQuery", s.subscribeQueryBlock()))
	}
}

func (s *Subscriber) processBlockEvent(ctx context.Context, tasks *queue.Queue[neutrontypes.RegisteredQuery]) error {
	// Get last block height.
	status, err := s.rpcClient.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Status: %w", err)
	}
	currentHeight := uint64(status.SyncInfo.LatestBlockHeight)

	for _, activeQuery := range s.activeQueries {
		// Skip the ActiveQuery if we didn't reach the update time.
		if currentHeight < (activeQuery.LastSubmittedResultLocalHeight + activeQuery.UpdatePeriod) {
			continue
		}

		// Send the query to the task queue.
		tasks.Enqueue(*activeQuery)

		// Set the LastSubmittedResultLocalHeight to the current height.
		activeQuery.LastSubmittedResultLocalHeight = currentHeight
	}

	return nil
}

// processUpdateEvent retrieves up-to-date information about each updated query and saves
// it to state. Note: an update event is emitted both on query creation and on query updates.
func (s *Subscriber) processUpdateEvent(ctx context.Context, event tmtypes.ResultEvent) error {
	ok, err := s.checkEvents(event)
	if err != nil {
		return fmt.Errorf("failed to checkEvents: %w", err)
	}
	if !ok {
		return nil
	}

	// There can be multiple events of the same type associated with our connection id in a
	// single tmtypes.ResultEvent value. We need to process all of them.
	var events = event.Events
	for idx := range events[connectionIdAttr] {
		var (
			owner   = events[ownerAttr][idx]
			queryID = events[queryIdAttr][idx]
		)
		if !s.isWatchedAddress(owner) {
			s.logger.Debug("Skipping query (wrong owner)", zap.String("owner", owner),
				zap.String("query_id", queryID))
			continue
		}

		// Load all information about the neutronQuery directly from ToNeutronRegisteredQuery.
		neutronQuery, err := s.getNeutronRegisteredQuery(ctx, queryID)
		if err != nil {
			return fmt.Errorf("failed to getNeutronRegisteredQuery: %w", err)
		}

		if !s.isWatchedMsgType(neutronQuery.QueryType) {
			s.logger.Debug("Skipping query (wrong type)", zap.String("owner", owner),
				zap.String("query_id", queryID))
			continue
		}

		// Save the updated neutronQuery information to state.
		s.activeQueries[queryID] = neutronQuery
	}

	return nil
}

// processRemoveEvent TODO(oopcode).
func (s *Subscriber) processRemoveEvent(event tmtypes.ResultEvent) error {
	ok, err := s.checkEvents(event)
	if err != nil {
		return fmt.Errorf("failed to checkEvents: %w", err)
	}
	if !ok {
		return nil
	}

	// There can be multiple events of the same type associated with our connection id in a
	// single tmtypes.ResultEvent value. We need to process all of them.
	var events = event.Events
	for idx := range events[connectionIdAttr] {
		var (
			owner   = events[ownerAttr][idx]
			queryID = events[queryIdAttr][idx]
		)
		if !s.isWatchedAddress(events[ownerAttr][idx]) {
			s.logger.Debug("Skipping query (wrong owner)", zap.String("owner", owner),
				zap.String("query_id", queryID))
			continue
		}

		// Delete the query from the active queries list.
		delete(s.activeQueries, queryID)
	}

	return nil
}
