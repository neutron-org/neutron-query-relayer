package proofer

import "github.com/tendermint/tendermint/proto/tendermint/crypto"

// TODO: use types from repo?
// StorageValue is basic building proof block
type StorageValue struct {
	Key    []byte
	Value  []byte
	Proofs []crypto.ProofOp // https://github.com/tendermint/tendermint/blob/29ad4dcb3b260ea7762bb307ae397833e1bd360a/proto/tendermint/crypto/proof.pb.go#L211
}

// Query has all the information for what we can collect Proof
type Query struct {
	SourceChainId string
	ConnectionId  string
	ChainId       string
	QueryId       string
	Type          string
	Params        map[string]string
}

type Proof struct {
}
