package raw

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authz "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
)

var (
	ModuleBasics = []module.AppModuleBasic{
		auth.AppModuleBasic{},
		authz.AppModuleBasic{},
		bank.AppModuleBasic{},
	}
)

type Codec struct {
	InterfaceRegistry types.InterfaceRegistry
	Marshaller        codec.ProtoCodecMarshaler
	Amino             *codec.LegacyAmino
}

func MakeCodecDefault() Codec {
	return MakeCodec(ModuleBasics)
}

// MakeCodec registers interfaces needed for serialization
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
		Amino:             codec.NewLegacyAmino(),
	}
}
