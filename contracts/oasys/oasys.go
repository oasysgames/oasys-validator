package oasys

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

const (
	// Address of contracts in genesis.
	EnvironmentAddress  = "0x0000000000000000000000000000000000001000"
	StakeManagerAddress = "0x0000000000000000000000000000000000001001"
	AllowListAddress    = "0x0000000000000000000000000000000000001002"

	// Address of initial wallet in genesis.
	mainnetGenesisWalletAddress = "0x54288e1CA4EDc0FB2cC6ea60AD189FAAF386dBc2"
	testnetGenesisWalletAddress = "0xBA6c8AB502D878aa250654820c579e71d14aF791"
)

var (
	GenesisHash        common.Hash
	defaultGenesisHash = common.Hash{}
)

// StateDB is an interface of state.StateDB.
type StateDB interface {
	GetCode(addr common.Address) []byte
	SetCode(addr common.Address, code []byte)
	SetState(addr common.Address, key common.Hash, value common.Hash)
}

// Deploy oasys built-in contracts.
func Deploy(chainConfig *params.ChainConfig, state StateDB, block uint64) {
	if chainConfig == nil || chainConfig.Oasys == nil || state == nil {
		return
	}

	deploy, ok := deployments[GenesisHash]
	if !ok {
		deploy = deployments[defaultGenesisHash]
	}

	if contracts, ok := deploy[block]; ok {
		for _, c := range contracts {
			c.deploy(state, block)
		}
	}
}
