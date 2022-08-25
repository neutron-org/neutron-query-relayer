package subscriber

import "github.com/neutron-org/neutron/x/interchainqueries/types"

// types of keys for parsing incoming events
const (
	// moduleAttr is the key of the message event's attribute that contains the module the event
	// originates from.
	moduleAttr = "message.module"
	// actionAttr is the key of the message event's attribute that contains the event's action.
	actionAttr = "message.action"
	// eventAttr is the key of the tendermint's event attribute that contains the kind of event.
	eventAttr = "tm.event"

	// zoneIdAttr is the key of the Neutron's custom message event's attribute that contains the
	// zone ID the event originates from.
	zoneIdAttr = "message." + types.AttributeKeyZoneID
	// queryIdAttr is the key of the Neutron's custom message event's attribute that contains the
	// incoming ICQ ID.
	queryIdAttr = "message." + types.AttributeKeyQueryID
	// kvKeyAttr is the key of the Neutron's custom message event's attribute that contains the KV
	// values for the incoming KV ICQ.
	kvKeyAttr = "message." + types.AttributeKeyKVQuery
	// transactionsFilter is the key of the Neutron's custom message event's attribute that contains
	// the transaction filter value for the incoming TX ICQ.
	transactionsFilterAttr = "message." + types.AttributeTransactionsFilterQuery
	// typeAttr is the key of the Neutron's custom message event's attribute that contains the type
	// of the incoming ICQ.
	typeAttr = "message." + types.AttributeKeyQueryType
	// ownerAttr is the key of the Neutron's custom message event's attribute that contains the
	// address of the ICQ owner.
	ownerAttr = "message." + types.AttributeKeyOwner
)
