package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/kv"
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
	pairs := kv.Pairs{}
	pairs.Unmarshal(abciResp.Response.GetValue())
	lens := make(map[int]uint64)
	for _, i := range pairs.Pairs {

		// https://github.com/cosmos/interchain-security/blob/ed04399147a0c3847645f3e68f41ea16db4c0f48/x/ccv/consumer/types/keys.go#L145
		maturedID := sdk.BigEndianToUint64(i.Key[9:17])
		maturityTime := sdk.BigEndianToUint64(i.Key[1:9]) // unixtime nanoseconds
		matured[maturedID] = time.Unix(0, int64(maturityTime))
	}
	fmt.Println(lens)

	fmt.Println(time.Now().Sub(tStart))
	return matured
}

func get_vsc_timestamps() (map[uint64]time.Time, uint64) {
	vscIDtoStorageKey := make(map[uint64]time.Time)
	tStart := time.Now()
	abciResp := get_raw_abci_response("https://rpc.cosmoshub.strange.love:443/abci_query?path=%22store/provider/subspace%22&data=0x12")
	pairs := kv.Pairs{}
	pairs.Unmarshal(abciResp.Response.GetValue())
	for _, i := range pairs.Pairs {

		// https://github.com/cosmos/interchain-security/blob/ed04399147a0c3847645f3e68f41ea16db4c0f48/x/ccv/provider/types/keys.go#L273
		chainIDlen := sdk.BigEndianToUint64(i.Key[1:9])
		chainID := string(i.Key[9 : 9+chainIDlen])
		vscID := sdk.BigEndianToUint64(i.Key[9+chainIDlen : 17+chainIDlen])
		if chainID != "neutron-1" {
			continue
		}
		var err error
		vscIDtoStorageKey[vscID], err = sdk.ParseTimeBytes(i.Value)
		if err != nil {
			log.Fatalln(err)
		}

	}
	fmt.Println(time.Now().Sub(tStart))
	return vscIDtoStorageKey, uint64(abciResp.Response.Height)
}

func main() {
	matured := get_matured_packets()
	fmt.Println("matured count: ", len(matured))
	ids, _ := get_vsc_timestamps()
	fmt.Println("neutron-1 timestamps count: ", len(ids))

	fmt.Println("Outdated packets!!!!!")
	for id, t := range ids {
		mTime, found := matured[id]
		if found {
			if t.Add(3024000000000000 * time.Nanosecond).Before(mTime) {
				fmt.Println("===============")
				fmt.Println("vscID: ", id)
				fmt.Println("timestamp: ", t)
				fmt.Println("timeout (+3024000000000000): ", t.Add(3024000000000000*time.Nanosecond))
				fmt.Println("maturity time: ", mTime)
				fmt.Println(t.Add(3024000000000000 * time.Nanosecond).Sub(mTime))
			}
		}
	}
	fmt.Println("Outdated list end!!!!!!!!!!!!!!!!")

	fmt.Println("Timeout packets(cosmos) with no matured(neutron)")
	for id, t := range ids {
		_, found := matured[id]
		if !found {
			fmt.Println("===============")
			fmt.Println("vscID: ", id)
			fmt.Println("timestamp: ", t)
			fmt.Println("timeout (+3024000000000000): ", t.Add(3024000000000000*time.Nanosecond))
		}
	}
	fmt.Println("Timeout packets(cosmos) with no matured(neutron) end !!!!!!!!!!!!!!!!!")

	//fmt.Println("closest 5 timeout with no matured")
	//type TimeoutVSC struct {
	//	Time time.Time
	//	ID   uint64
	//}
	//pList := make([]TimeoutVSC, 0, len(ids))
	//for id, t := range ids {
	//	pList = append(pList, TimeoutVSC{
	//		Time: t,
	//		ID:   id,
	//	})
	//}
	//sort.Slice(pList, func(i, j int) bool { return pList[i].Time.Before(pList[j].Time) })
	//c = 0
	//for _, tvsc := range pList {
	//	_, found := matured[tvsc.ID]
	//	if !found {
	//		c++
	//		fmt.Println(tvsc)
	//	}
	//	if c > 5 {
	//		break
	//	}
	//}
	//fmt.Println(time.Now().Sub(t1))
}
