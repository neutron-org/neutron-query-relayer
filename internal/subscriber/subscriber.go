package subscriber

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/neutron-org/neutron-query-relayer/internal/raw"

	"github.com/tendermint/tendermint/rpc/client/http"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	restclient "github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

var (
	rpcWSEndpoint      = "/websocket"
	unsubscribeTimeout = time.Second * 5
)

// NewSubscriber creates a new Subscriber instance ready to subscribe on the given chain's events.
func NewSubscriber(
	rpcAddress string,
	restAddress string,
	connectionID string,
	registry *registry.Registry,
	watchedTypes []neutrontypes.InterchainQueryType,
	logger *zap.Logger,
) (*Subscriber, error) {
	// rpcClient is used to subscribe to Neutron events.
	rpcClient, err := http.New(rpcAddress, rpcWSEndpoint)
	if err != nil {
		return nil, fmt.Errorf("could not create new tendermint rpcClient: %w", err)
	}

	if err = rpcClient.Start(); err != nil {
		return nil, fmt.Errorf("could not start tendermint rpcClient: %w", err)
	}

	// restClient is used to retrieve registered queries from Neutron.
	restClient, err := raw.NewRESTClient(restAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get newRESTClient: %w", err)
	}

	// Contains the types of queries that we are ready to serve (KV / TX).
	watchedTypesMap := make(map[neutrontypes.InterchainQueryType]struct{})
	for _, queryType := range watchedTypes {
		watchedTypesMap[queryType] = struct{}{}
	}

	return &Subscriber{
		rpcClient:  rpcClient,
		restClient: restClient,

		rpcAddress:   rpcAddress,
		connectionID: connectionID,
		registry:     registry,
		logger:       logger,
		watchedTypes: watchedTypesMap,

		activeQueries: map[string]*neutrontypes.RegisteredQuery{},
	}, nil
}

// Subscriber is responsible for subscribing on chain's ICQ events. It parses incoming events,
// filters them in accordance with the Registry configuration and watchedTypes, and provides a
// stream of split to KV and TX messages.
type Subscriber struct {
	rpcClient  *http.HTTP                 // Used to subscribe to events
	restClient *restclient.HTTPAPIConsole // Used to run Neutron-specific queries using the REST

	rpcAddress   string
	connectionID string
	registry     *registry.Registry
	logger       *zap.Logger
	watchedTypes map[neutrontypes.InterchainQueryType]struct{}

	activeQueries map[string]*neutrontypes.RegisteredQuery
}

// Subscribe subscribes to 3 types of events: 1. a new block was created, 2. a query was updated (created / updated),
// 3. a query was removed.
func (s *Subscriber) Subscribe(ctx context.Context, tasks chan neutrontypes.RegisteredQuery) error {
	queries, err := s.getNeutronRegisteredQueries(ctx)
	if err != nil {
		return fmt.Errorf("could not getNeutronRegisteredQueries: %w", err)
	}
	s.activeQueries = queries

	// Make sure we try to unsubscribe from events if an error occurs.
	defer s.unsubscribe()

	updateEvents, err := s.rpcClient.Subscribe(ctx, s.subscriberName(), s.getQueryUpdatedSubscription())
	if err != nil {
		return fmt.Errorf("could not subscribe to events: %w", err)
	}

	removeEvents, err := s.rpcClient.Subscribe(ctx, s.subscriberName(), s.getQueryRemovedSubscription())
	if err != nil {
		return fmt.Errorf("could not subscribe to events: %w", err)
	}

	blockEvents, err := s.rpcClient.Subscribe(ctx, s.subscriberName(), s.getNewBlockHeaderSubscription())
	if err != nil {
		return fmt.Errorf("could not subscribe to events: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Context cancelled, shutting down subscriber...")
			return nil
		case <-blockEvents:
			if err := s.processBlockEvent(ctx, tasks); err != nil {
				return fmt.Errorf("failed to processBlockEvent: %w", err)
			}
		case event := <-updateEvents:
			if err = s.processUpdateEvent(ctx, event); err != nil {
				return fmt.Errorf("failed to processUpdateEvent: %w", err)
			}
		case event := <-removeEvents:
			if err = s.processRemoveEvent(event); err != nil {
				return fmt.Errorf("failed to processRemoveEvent: %w", err)
			}
		}
	}
}

func (s *Subscriber) processBlockEvent(ctx context.Context, tasks chan neutrontypes.RegisteredQuery) error {
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

		// Send the query to the tasks queue.
		tasks <- *activeQuery

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

		// Load all information about the neutronQuery directly from Neutron.
		neutronQuery, err := s.getNeutronRegisteredQuery(ctx, queryID)
		if err != nil {
			return fmt.Errorf("failed to getNeutronRegisteredQuery: %w", err)
		}

		if !s.isWatchedMsgType(neutronQuery.QueryType) {
			s.logger.Debug("Skipping query (wrong type)", zap.String("owner", owner),
				zap.String("query_id", queryID))
			continue
		}

		// Save the updated query information to memory.
		s.activeQueries[queryID] = neutronQuery
	}

	return nil
}

// processRemoveEvent deletes an event that was removed on Neutron from memory.
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
			queryID = events[queryIdAttr][idx]
		)

		// Delete the query from the active queries list.
		delete(s.activeQueries, queryID)
	}

	return nil
}

// unsubscribes from all previously registered subscriptions. Please note that
// this method does not return an error and does not panic.
func (s *Subscriber) unsubscribe() {
	ctx, cancel := context.WithTimeout(context.Background(), unsubscribeTimeout)
	defer cancel()

	var (
		wg            = &sync.WaitGroup{}
		subscriptions = []string{
			s.getQueryUpdatedSubscription(),
			s.getQueryRemovedSubscription(),
			s.getNewBlockHeaderSubscription(),
		}
	)
	for _, subscription := range subscriptions {
		wg.Add(1)
		go func(subscription string) {
			defer wg.Done()

			if err := s.rpcClient.Unsubscribe(ctx, s.subscriberName(), subscription); err != nil {
				s.logger.Error("failed to Unsubscribe from tm events",
					zap.Error(err), zap.String("subscription", subscription))
			}

			s.logger.Debug("unsubscribed", zap.String("subscription", subscription))
		}(subscription)
	}

	wg.Wait()
}
