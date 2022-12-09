package subscriber

import (
	"github.com/neutron-org/neutron/x/interchainqueries/types"
)

const eventTypePrefix = types.EventTypeNeutronMessage

// types of keys for parsing incoming events
const (
	// moduleAttr is the key of the message event's attribute that contains the module the event
	// originates from.
	moduleAttr = eventTypePrefix + ".module"
	// actionAttr is the key of the message event's attribute that contains the event's action.
	actionAttr = eventTypePrefix + ".action"
	// eventAttr is the key of the tendermint's event attribute that contains the kind of event.
	eventAttr = "tm.event"

	// connectionIdAttr is the key of the Neutron's custom message event's attribute that contains the
	// connectionID of the event's ActiveQuery.
	connectionIdAttr = eventTypePrefix + "." + types.AttributeKeyConnectionID
	// queryIdAttr is the key of the Neutron's custom message event's attribute that contains the
	// incoming ICQ ID.
	queryIdAttr = eventTypePrefix + "." + types.AttributeKeyQueryID
	// kvKeyAttr is the key of the Neutron's custom message event's attribute that contains the KV
	// values for the incoming KV ICQ.
	kvKeyAttr = eventTypePrefix + "." + types.AttributeKeyKVQuery
	// transactionsFilterAttr is the key of the Neutron's custom message event's attribute that
	// contains the transaction filter value for the incoming TX ICQ.
	transactionsFilterAttr = eventTypePrefix + "." + types.AttributeTransactionsFilterQuery
	// typeAttr is the key of the Neutron's custom message event's attribute that contains the type
	// of the incoming ICQ.
	typeAttr = eventTypePrefix + "." + types.AttributeKeyQueryType
	// ownerAttr is the key of the Neutron's custom message event's attribute that contains the
	// address of the ICQ owner.
	ownerAttr = eventTypePrefix + "." + types.AttributeKeyOwner
)
