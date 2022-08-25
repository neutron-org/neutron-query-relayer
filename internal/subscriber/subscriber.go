package subscriber

import (
	"context"
	"fmt"
	"strconv"

	"github.com/neutron-org/cosmos-query-relayer/internal/registry"
	"github.com/neutron-org/cosmos-query-relayer/internal/relay"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"github.com/tendermint/tendermint/rpc/client/http"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
)

// NewSubscriber creates a new Subscriber instance ready to subscribe on the given chain's events.
func NewSubscriber(rpcAddress, targetChainID string, registry *registry.Registry, logger *zap.Logger) (*Subscriber, error) {
	client, err := http.New(rpcAddress, "/websocket")
	if err != nil {
		return nil, fmt.Errorf("could not create new tendermint client: %w", err)
	}
	err = client.Start()
	if err != nil {
		return nil, fmt.Errorf("could not start tendermint client: %w", err)
	}
	return &Subscriber{
		client:        client,
		rpcAddress:    rpcAddress,
		targetChainID: targetChainID,
		registry:      registry,
		logger:        logger,
	}, nil
}

// Subscriber is responsible for subscribing on chain's ICQ events. It parses incoming events,
// filters them in accordance with the Registry configuration, and provides a stream of split
// to KV and TX messages.
type Subscriber struct {
	client        *http.HTTP
	rpcAddress    string
	targetChainID string
	registry      *registry.Registry
	logger        *zap.Logger
}

// Subscribe subscribes on chain's events, transforms them to KV and TX messages and sends to
// the corresponding channels. The subscriber can be stopped by closing the context.
func (s *Subscriber) Subscribe(ctx context.Context) (<-chan *relay.MessageKV, <-chan *relay.MessageTX, error) {
	subscriberName := s.subscriberName()
	subscribeQuery := s.subscribeQuery()
	events, err := s.client.Subscribe(ctx, subscriberName, subscribeQuery)
	if err != nil {
		return nil, nil, fmt.Errorf("could not subscribe to events: %w", err)
	}
	s.logger.Debug("subscriber has subscribed on neutron events", zap.String("query", subscribeQuery))

	kvChan := make(chan *relay.MessageKV)
	txChan := make(chan *relay.MessageTX)
	unsubscribe := func() {
		_ = s.client.Unsubscribe(context.Background(), subscriberName, subscribeQuery)
		_ = s.client.Stop()
		close(kvChan)
		close(txChan)
		s.logger.Debug("subscriber has been stopped")
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				unsubscribe()
				return
			case event := <-events:
				s.logger.Debug("received an event from Neutron")
				msgs, err := s.extractMessages(event)
				if err != nil {
					s.logger.Error("failed to extract messages from incoming event", zap.Error(err))
					continue
				}
				if len(msgs) == 0 {
					s.logger.Debug("event has been skipped: it's not intended for us", zap.String("query", event.Query))
					continue
				}

				s.logger.Debug("handling event from Neutron", zap.Int("messages", len(msgs)))
				for _, msg := range msgs {
					s.logger.Debug("handling a message from event", zap.String("message_type", string(msg.messageType)))
					switch msg.messageType {
					case neutrontypes.InterchainQueryTypeKV:
						select {
						case kvChan <- buildMessageKV(msg):
							s.logger.Debug("a KV message sent from subscriber to consumer")
						case <-ctx.Done():
							unsubscribe()
							return
						}
					case neutrontypes.InterchainQueryTypeTX:
						select {
						case txChan <- buildMessageTX(msg):
							s.logger.Debug("a TX message sent from subscriber to consumer")
						case <-ctx.Done():
							unsubscribe()
							return
						}
					}
				}
			}
		}
	}()
	return kvChan, txChan, nil
}

// subscribeQuery returns the subscriber name.
func (s *Subscriber) subscriberName() string {
	return s.targetChainID + "-client"
}

// subscribeQuery returns a query to filter out interchain query events.
func (s *Subscriber) subscribeQuery() string {
	return fmt.Sprintf("%s='%s' AND %s='%s' AND %s='%s' AND %s='%s'",
		zoneIdAttr, s.targetChainID,
		moduleAttr, neutrontypes.ModuleName,
		actionAttr, neutrontypes.AttributeValueQuery,
		eventAttr, types.EventNewBlockHeader,
	)
}

// extractMessages validates events received from the target chain, filters them, and transforms
// to a slice of queryEventMessage.
func (s *Subscriber) extractMessages(e tmtypes.ResultEvent) ([]*queryEventMessage, error) {
	events := e.Events
	icqEventsCount := len(events[zoneIdAttr])
	if icqEventsCount == 0 {
		return nil, nil
	}

	if len(events[kvKeyAttr]) != icqEventsCount ||
		len(events[transactionsFilterAttr]) != icqEventsCount ||
		len(events[queryIdAttr]) != icqEventsCount ||
		len(events[typeAttr]) != icqEventsCount {
		return nil, fmt.Errorf("events attributes length does not match for events=%v", events)
	}

	messages := make([]*queryEventMessage, 0, icqEventsCount)
	for idx := range events[zoneIdAttr] {
		if !s.isWatchedAddress(events[ownerAttr][idx]) {
			continue
		}

		queryIdStr := events[queryIdAttr][idx]
		queryId, err := strconv.ParseUint(queryIdStr, 10, 64)
		if err != nil {
			s.logger.Debug("failed to parse query ID as uint", zap.String("query_id", queryIdStr), zap.Error(err))
			continue
		}

		messageType := neutrontypes.InterchainQueryType(events[typeAttr][idx])
		switch messageType {
		case neutrontypes.InterchainQueryTypeKV:
			kvKeyAttrVal := events[kvKeyAttr][idx]
			kvKeys, err := neutrontypes.KVKeysFromString(kvKeyAttrVal)
			if err != nil {
				s.logger.Debug(fmt.Sprintf("invalid %s attr", kvKeyAttr), zap.String(kvKeyAttr, kvKeyAttrVal), zap.Error(err))
				continue
			}
			messages = append(messages, &queryEventMessage{
				queryId:     queryId,
				messageType: messageType,
				kvKeys:      kvKeys,
			})
		case neutrontypes.InterchainQueryTypeTX:
			messages = append(messages, &queryEventMessage{
				queryId:            queryId,
				messageType:        messageType,
				transactionsFilter: events[transactionsFilterAttr][idx],
			})
		default:
			s.logger.Debug("unknown query_type", zap.String("query_type", string(messageType)))
		}
	}
	return messages, nil
}

// isWatchedAddress returns true if the address is within the registry watched addresses or there
// are no registry watched addresses configured for the subscriber meaning all addresses are watched.
func (s *Subscriber) isWatchedAddress(address string) bool {
	return s.registry.IsEmpty() || s.registry.Contains(address)
}
