package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ccv "github.com/cosmos/interchain-security/x/ccv/types"
	"github.com/neutron-org/neutron-query-relayer/internal/raw"
	"github.com/neutron-org/neutron-query-relayer/internal/tmquerier"
	"github.com/neutron-org/neutron/x/interchainqueries/types"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	jsontypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"log"
	"net/http"
	"time"
)

func PendingVSCsKey(chainID string) []byte {
	return append([]byte{17}, []byte(chainID)...)
}

func HeightValsetUpdateIDKey(height uint64) []byte {
	hBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(hBytes, height)
	return append([]byte{13}, hBytes...)
}

//binary.BigEndian.Uint64(bz)

func getvalsetstatus() {
	fmt.Println("starting")
	rpc := "http://65.109.157.115:26657"
	chainid := "neutron-1"
	valaccprefix := "neutronvaloper"
	height := 15256100
	key := types.KVKey{
		Path: "consumer",
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
	//if err := packets.Unmarshal(value.Value); err != nil {
	//	// An error here would indicate something is very wrong,
	//	// the PendingVSCPackets are assumed to be correctly serialized in AppendPendingVSCPackets.
	//	panic(fmt.Errorf("cannot unmarshal pending validator set changes: %w", err))
	//}
	fmt.Println(value, h)
}

func getvalset() {
	var packets ccv.ValidatorSetChangePackets
	fmt.Println("starting")
	rpc := "http://65.109.157.115:26657"
	chainid := "cosmoshub-4"
	valaccprefix := "cosmosvaloper"
	height := 15256100
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

func get_raw_abci_response(url string) *coretypes.ResultABCIQuery {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.StatusCode)
	var abciResp coretypes.ResultABCIQuery
	response := &jsontypes.RPCResponse{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&response)
	if err != nil {
		log.Fatalf("failed to decode response body: %v", err)
	}
	err = json.Unmarshal(response.Result, &abciResp)
	if err != nil {
		log.Fatalf("failed to decode abciResp: %v", err)
	}
	return &abciResp
}

func get_matured_packets() map[uint64]time.Time {
	matured := make(map[uint64]time.Time)
	tStart := time.Now()
	abciResp := get_raw_abci_response("https://rpc-kralum.neutron-1.neutron.org/abci_query?path=%22store/ccvconsumer/subspace%22&data=0x0c")
	sep := []byte{10, 39, 10, 17, 12}
	splitted := bytes.Split(abciResp.Response.GetValue(), sep)
	//c := 0
	for _, i := range splitted {
		if len(i) == 0 {
			continue
		}
		// https://github.com/cosmos/interchain-security/blob/ed04399147a0c3847645f3e68f41ea16db4c0f48/x/ccv/consumer/types/keys.go#L145
		maturedID := sdk.BigEndianToUint64(i[8:16])
		maturityTime := sdk.BigEndianToUint64(i[0:8]) // unixtime nanoseconds
		matured[maturedID] = time.Unix(0, int64(maturityTime))
	}
	fmt.Println(time.Now().Sub(tStart))
	return matured
}

func get_vsc_timestamps() (map[uint64]time.Time, uint64) {
	vscIDtoStorageKey := make(map[uint64]time.Time)
	tStart := time.Now()
	abciResp := get_raw_abci_response("https://rpc.cosmoshub.strange.love:443/abci_query?path=%22store/provider/subspace%22&data=0x12")
	sep := []byte{10, 59, 10, 26, 18}
	splitted := bytes.Split(abciResp.Response.GetValue(), sep)
	//fmt.Println(len(splitted))
	//fmt.Println(abciResp.Response.GetValue()[:200])
	for _, i := range splitted {
		if len(i) == 0 {
			continue
		}

		// https://github.com/cosmos/interchain-security/blob/ed04399147a0c3847645f3e68f41ea16db4c0f48/x/ccv/provider/types/keys.go#L273
		chainIDlen := sdk.BigEndianToUint64(i[0:8])
		chainID := string(i[8 : 8+chainIDlen])
		vscID := sdk.BigEndianToUint64(i[8+chainIDlen : 16+chainIDlen])
		if chainID != "neutron-1" {
			continue
		}
		var err error
		vscIDtoStorageKey[vscID], err = sdk.ParseTimeBytes(i[16+chainIDlen+2:])
		if err != nil {
			log.Fatalln(err)
		}
		//c++
		//if c > 10 {
		//	break
		//}
	}
	fmt.Println(time.Now().Sub(tStart))
	return vscIDtoStorageKey, uint64(abciResp.Response.Height)
}

func getVSCTimestamp(height uint64, storageKey []byte) time.Time {
	fmt.Println("starting")
	rpc := "https://rpc.cosmoshub.strange.love:443"
	chainid := "cosmoshub-4"
	valaccprefix := "cosmosvaloper"
	key := types.KVKey{
		Path: "provider",
		Key:  storageKey,
	}
	targetClient, err := raw.NewRPCClient(rpc, time.Second*10)
	if err != nil {
		log.Fatalf("could not initialize target rpc client: %w", err)
	}

	targetQuerier, err := tmquerier.NewQuerier(targetClient, chainid, valaccprefix)
	if err != nil {
		log.Fatalf("cannot connect to target chain: %w", err)
	}

	value, _, err := targetQuerier.QueryTendermintProof(context.Background(), int64(height), key.GetPath(), key.GetKey())
	if err != nil {
		log.Fatalf("failed to query tendermint proof for path=%s and key=%v: %w", key.GetPath(), key.GetKey(), err)
	}
	fmt.Println(value.Value)
	t, err := sdk.ParseTimeBytes(value.Value)
	if err != nil {
		log.Fatalln(err)
	}

	return t //.Add(3024000000000000 * time.Nanosecond)
}

func main() {
	t1 := time.Now()
	matured := get_matured_packets()
	fmt.Println("matured count: ", len(matured))
	ids, _ := get_vsc_timestamps()
	fmt.Println("neutron-1 timestamps count: ", len(ids))
	c := 0
	for id, t := range ids {
		fmt.Println("===========")
		fmt.Println("vscID: ", id)
		fmt.Println("timestamp: ", t)
		fmt.Println("timeout (+3024000000000000): ", t.Add(3024000000000000*time.Nanosecond))
		mTime, found := matured[id]
		if !found {
			fmt.Println("no maturity found")
		} else {
			fmt.Println("maturity time: ", mTime)
			fmt.Println(t.Add(3024000000000000 * time.Nanosecond).Sub(matured[id]))
		}
		c++
		if c > 10 {
			break
		}
	}
	fmt.Println(time.Now().Sub(t1))
}

//func main() {
//	b := []byte{18, 29, 50, 48, 50, 51, 45, 48, 54, 45, 48, 50, 84, 49, 50, 58, 52, 52, 58, 53, 56, 46, 51, 50, 52, 54, 48, 48, 55, 50, 52}
//                        50, 48, 50, 51, 45, 48, 54, 45, 48, 55, 84, 50, 50, 58, 52, 53, 58, 50, 57, 46, 51, 48, 52, 54, 56, 53, 52, 49, 52
//}
