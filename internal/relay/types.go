package relay

type queryEventMessage struct {
	queryId     uint64
	messageType string
	parameters  string
}

type getDelegatorDelegationsParams struct {
	Delegator string `json:"delegator"`
}

type getAllBalancesParams struct {
	Addr  string `json:"addr"`
	Denom string `json:"denom"`
}

type recipientTransactionsParams map[string]string

type exchangeRateParams struct {
	// GetSupply part
	Denom string `json:"denom"`

	// GetDelegatorDelegations part
	Delegator string `json:"delegator"`
}
