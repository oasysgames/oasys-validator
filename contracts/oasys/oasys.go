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
	mainnetGenesisWalletAddress = "0xdF3548cD5e355202AE92e766c7361eA4F6687A61"
	testnetGenesisWalletAddress = "0xbf9Ec8a822519C00128f0c7C13f13cafF0501Aea"
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
