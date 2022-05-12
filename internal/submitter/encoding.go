package submitter

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	authz "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
)

type Codec struct {
	InterfaceRegistry types.InterfaceRegistry
	Marshaller        codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

func MakeCodecDefault() Codec {
	return MakeCodec(ModuleBasics)
}

func MakeCodec(moduleBasics []module.AppModuleBasic) Codec {
	modBasic := module.NewBasicManager(moduleBasics...)
	encodingConfig := MakeCodecConfig()
	std.RegisterLegacyAminoCodec(encodingConfig.Amino)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	modBasic.RegisterLegacyAminoCodec(encodingConfig.Amino)
	modBasic.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	return encodingConfig
}

func MakeCodecConfig() Codec {
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaller := codec.NewProtoCodec(interfaceRegistry)
	return Codec{
		InterfaceRegistry: interfaceRegistry,
		Marshaller:        marshaller,
		TxConfig:          tx.NewTxConfig(marshaller, tx.DefaultSignModes),
		Amino:             codec.NewLegacyAmino(),
	}
}

var (
	// TODO: do we need these?
	ModuleBasics = []module.AppModuleBasic{
		auth.AppModuleBasic{},
		authz.AppModuleBasic{},
		bank.AppModuleBasic{},
		//transfer.AppModuleBasic{},
	}
)
