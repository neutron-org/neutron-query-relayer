package subscriber

import (
	"context"
	"fmt"
	"strconv"

	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"github.com/tendermint/tendermint/rpc/client/http"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
)

// NewSubscriber creates a new Subscriber instance ready to subscribe on the given chain's events.
func NewSubscriber(
	rpcAddress string,
	targetChainID string,
	registry *registry.Registry,
	watchedTypes []neutrontypes.InterchainQueryType,
	logger *zap.Logger,
) (*Subscriber, error) {
	client, err := http.New(rpcAddress, "/websocket")
	if err != nil {
		return nil, fmt.Errorf("could not create new tendermint client: %w", err)
	}
	if err = client.Start(); err != nil {
		return nil, fmt.Errorf("could not start tendermint client: %w", err)
	}
	watchedTypesMap := make(map[neutrontypes.InterchainQueryType]struct{})
	for _, queryType := range watchedTypes {
		watchedTypesMap[queryType] = struct{}{}
	}
	return &Subscriber{
		client:        client,
		rpcAddress:    rpcAddress,
		targetChainID: targetChainID,
		registry:      registry,
		logger:        logger,
		watchedTypes:  watchedTypesMap,
	}, nil
}

// Subscriber is responsible for subscribing on chain's ICQ events. It parses incoming events,
// filters them in accordance with the Registry configuration and watchedTypes, and provides a
// stream of split to KV and TX messages.
type Subscriber struct {
	client        *http.HTTP
	rpcAddress    string
	targetChainID string
	registry      *registry.Registry
	logger        *zap.Logger
	watchedTypes  map[neutrontypes.InterchainQueryType]struct{}
	subCancel     context.CancelFunc // subCancel controls subscriber event handling loop
}

// Subscribe subscribes on chain's events, transforms them to KV and TX messages and sends to
// the corresponding channels. The subscriber can be stopped by closing the context. Message
// channels are never closed to prevent listeners from seeing an erroneous message.
func (s *Subscriber) Subscribe() (<-chan *relay.MessageKV, <-chan *relay.MessageTX, error) {
	events, err := s.client.Subscribe(context.Background(), s.subscriberName(), s.subscribeQuery())
	if err != nil {
		return nil, nil, fmt.Errorf("could not subscribe to events: %w", err)
	}
	s.logger.Debug("subscriber has subscribed to neutron events", zap.String("query", s.subscribeQuery()))

	subCtx, subCancel := context.WithCancel(context.Background())
	s.subCancel = subCancel
	kvChan := make(chan *relay.MessageKV)
	txChan := make(chan *relay.MessageTX)
	go func() {
		for {
			select {
			case <-subCtx.Done():
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
					if !s.isWatchedMsgType(msg.messageType) {
						s.logger.Debug(
							"message skipped due to submitter watched msg types configuration",
							zap.String("msg_type", string(msg.messageType)),
							zap.Uint64("query_id", msg.queryId),
						)
						continue
					}

					s.logger.Debug("handling a message from event", zap.String("message_type", string(msg.messageType)))
					switch msg.messageType {
					case neutrontypes.InterchainQueryTypeKV:
						select {
						case kvChan <- buildMessageKV(msg):
							s.logger.Debug("a KV message sent from subscriber to consumer")
						case <-subCtx.Done():
							return
						}
					case neutrontypes.InterchainQueryTypeTX:
						select {
						case txChan <- buildMessageTX(msg):
							s.logger.Debug("a TX message sent from subscriber to consumer")
						case <-subCtx.Done():
							return
						}
					}
				}
			}
		}
	}()
	return kvChan, txChan, nil
}

// Unsubscribe stops subscription and closes the subscriber client.
func (s *Subscriber) Unsubscribe() error {
	s.subCancel()
	if err := s.client.Unsubscribe(context.Background(), s.subscriberName(), s.subscribeQuery()); err != nil {
		return err
	}
	if err := s.client.Stop(); err != nil {
		return err
	}
	return nil
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

// isWatchedMsgType returns true if the given message type was added to the subscriber's watched
// query types list.
func (s *Subscriber) isWatchedMsgType(msgType neutrontypes.InterchainQueryType) bool {
	_, ex := s.watchedTypes[msgType]
	return ex
}

// isWatchedAddress returns true if the address is within the registry watched addresses or there
// are no registry watched addresses configured for the subscriber meaning all addresses are watched.
func (s *Subscriber) isWatchedAddress(address string) bool {
	return s.registry.IsEmpty() || s.registry.Contains(address)
}
