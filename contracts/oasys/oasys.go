package oasys

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

const (
	// Built-in contract prefixes.
	BuiltInContractPrefix1 = "0x0000000000000000" // 8 bytes
	BuiltInContractPrefix2 = "0x5200000000000000" // 8 bytes

	// Address of contracts in genesis.
	EnvironmentAddress  = BuiltInContractPrefix1 + "000000000000000000001000"
	StakeManagerAddress = BuiltInContractPrefix1 + "000000000000000000001001"
	AllowListAddress    = BuiltInContractPrefix1 + "000000000000000000001002"

	// Address of initial wallet in genesis.
	mainnetGenesisWalletAddress = "0xdF3548cD5e355202AE92e766c7361eA4F6687A61"
	testnetGenesisWalletAddress = "0xbf9Ec8a822519C00128f0c7C13f13cafF0501Aea"

	// Address of contracts in `oasys-governance-contract`.
	EVMAccessControl = BuiltInContractPrefix2 + "00000000000000000000003F"
)

var (
	GenesisHash,
	defaultGenesisHash common.Hash
)

// Interface that extracts necessary methods from vm.StateDB
type StateDB interface {
	GetCode(addr common.Address) []byte
	SetCode(addr common.Address, code []byte) (prev []byte)
	SetState(addr common.Address, key, value common.Hash) (prev common.Hash)
	SetNonce(common.Address, uint64, tracing.NonceChangeReason)
}

// Deploy built-in contracts.
func Deploy(chainConfig *params.ChainConfig, state StateDB,
	blockNumber *big.Int, lastBlockTime, blockTime uint64) {
	if lastBlockTime >= blockTime {
		panic("lastBlockTime should be less than currentBlockTime")
	}
	if chainConfig == nil || chainConfig.Oasys == nil || state == nil {
		return
	}

	// Deploy built-in contracts when the block number reaches the specified block height.
	deploymentMap, ok := deploymentSets[GenesisHash]
	if !ok {
		deploymentMap = deploymentSets[defaultGenesisHash]
	}
	if deploymentSet, ok := deploymentMap[blockNumber.Uint64()]; ok {
		for _, deployments := range deploymentSet {
			for _, d := range deployments {
				d.deploy(chainConfig, state, blockNumber)
			}
		}
	}

	// Deploy EIP-2935 block hash history contract
	if chainConfig.IsOnPrague(blockNumber, lastBlockTime, blockTime) {
		state.SetCode(params.HistoryStorageAddress, params.HistoryStorageCode)
		state.SetNonce(params.HistoryStorageAddress, 1, tracing.NonceChangeNewContract)
		log.Info("Set code for HistoryStorageAddress", "blockNumber", blockNumber, "blockTime", blockTime)
	}
}
