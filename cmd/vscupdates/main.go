package main

import (
	"context"
	"fmt"
	ccv "github.com/cosmos/interchain-security/x/ccv/types"
	"github.com/neutron-org/neutron-query-relayer/internal/raw"
	"github.com/neutron-org/neutron-query-relayer/internal/tmquerier"
	"github.com/neutron-org/neutron/x/interchainqueries/types"
	"log"
	"time"
)

func PendingVSCsKey(chainID string) []byte {
	return append([]byte{17}, []byte(chainID)...)
}

func main() {

	var packets ccv.ValidatorSetChangePackets
	fmt.Println("starting")
	rpc := "http://65.109.157.115:26657"
	chainid := "cosmoshub-4"
	valaccprefix := "cosmosvaloper"
	height := 15254400
	key := types.KVKey{
		Path: "provider",
		Key:  PendingVSCsKey("neutron-1"),
	}
	targetClient, err := raw.NewRPCClient(rpc, time.Second*10)
	if err != nil {
		log.Fatalf("could not initialize target rpc client: %w", err)
	}

	targetQuerier, err := tmquerier.NewQuerier(targetClient, chainid, valaccprefix)
	if err != nil {
		log.Fatalf("cannot connect to target chain: %w", err)
	}

	value, h, err := targetQuerier.QueryTendermintProof(context.Background(), int64(height), key.GetPath(), key.GetKey())
	if err != nil {
		log.Fatalf("failed to query tendermint proof for path=%s and key=%v: %w", key.GetPath(), key.GetKey(), err)
	}
	if err := packets.Unmarshal(value.Value); err != nil {
		// An error here would indicate something is very wrong,
		// the PendingVSCPackets are assumed to be correctly serialized in AppendPendingVSCPackets.
		panic(fmt.Errorf("cannot unmarshal pending validator set changes: %w", err))
	}
	fmt.Println(value, h)
	fmt.Println(packets.GetList())
	fmt.Println(len(packets.GetList()))
}
