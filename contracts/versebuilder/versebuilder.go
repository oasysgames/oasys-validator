package versebuilder

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts"
	"github.com/ethereum/go-ethereum/core/state"
)

var (
	addressPrefix         = contracts.AddressPrefix["versebuilder"].Hash().Big()
	l1BuildParamAddress   = common.BigToAddress(new(big.Int).Add(addressPrefix, big.NewInt(4096*1)))
	l1BuildDepositAddress = common.BigToAddress(new(big.Int).Add(addressPrefix, big.NewInt(4096*2)))
	l1BuildAgentAddress   = common.BigToAddress(new(big.Int).Add(addressPrefix, big.NewInt(4096*3)))
	l1BuildStep1Address   = common.BigToAddress(new(big.Int).Add(addressPrefix, big.NewInt(4096*4)))
	l1BuildStep2Address   = common.BigToAddress(new(big.Int).Add(addressPrefix, big.NewInt(4096*5)))
	l1BuildStep3Address   = common.BigToAddress(new(big.Int).Add(addressPrefix, big.NewInt(4096*6)))
	l1BuildStep4Address   = common.BigToAddress(new(big.Int).Add(addressPrefix, big.NewInt(4096*7)))

	l1BuildParam = &contracts.Contract{
		Address: l1BuildParamAddress,
		Code:    l1BuildParamCode,
		FixedStorage: map[string]interface{}{
			// uint256 public maxTransactionGasLimit
			"0x00": big.NewInt(15_000_000),
			// uint256 public l2GasDiscountDivisor
			"0x01": big.NewInt(32),
			// uint256 public enqueueGasCost
			"0x02": big.NewInt(60_000),
			// uint256 public fraudProofWindow
			"0x03": common.Big0,
			// uint256 public sequencerPublishWindow
			"0x04": common.Big0,
		},
		DynamicStorage: map[string]string{
			// bytes public l1StandardBridgeCode
			"0x05": l1StandardBridgeCode,
			// bytes public l1ERC721BridgeCode
			"0x06": l1ERC721BridgeCode,
		},
	}
	l1BuildDeposit = &contracts.Contract{
		Address: l1BuildDepositAddress,
		Code:    l1BuildDepositCode,
		FixedStorage: map[string]interface{}{
			// uint256 public requiredAmount
			"0x00": common.Big0,
			// uint256 public lockedBlock
			"0x01": common.Big0,
			// address public agentAddress
			"0x02": l1BuildAgentAddress,
		},
	}
	l1BuildAgent = &contracts.Contract{
		Address: l1BuildAgentAddress,
		Code:    l1BuildAgentCode,
		FixedStorage: map[string]interface{}{
			// address public depositAddress
			"0x00": l1BuildDepositAddress,
			// address public step1Address
			"0x01": l1BuildStep1Address,
			// address public step2Address
			"0x02": l1BuildStep2Address,
			// address public step3Address
			"0x03": l1BuildStep3Address,
			// address public step4Address
			"0x04": l1BuildStep4Address,
		},
	}
	l1BuildStep1 = &contracts.Contract{
		Address: l1BuildStep1Address,
		Code:    l1BuildStep1Code,
		FixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
		},
	}
	l1BuildStep2 = &contracts.Contract{
		Address: l1BuildStep2Address,
		Code:    l1BuildStep2Code,
		FixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
			// address public paramAddress
			"0x01": l1BuildParamAddress,
		},
	}
	l1BuildStep3 = &contracts.Contract{
		Address: l1BuildStep3Address,
		Code:    l1BuildStep3Code,
		FixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
		},
	}
	l1BuildStep4 = &contracts.Contract{
		Address: l1BuildStep4Address,
		Code:    l1BuildStep4Code,
		FixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
			// address public paramAddress
			"0x01": l1BuildParamAddress,
		},
	}

	l1BuilderContracts = []*contracts.Contract{
		l1BuildParam,
		l1BuildDeposit,
		l1BuildAgent,
		l1BuildStep1,
		l1BuildStep2,
		l1BuildStep3,
		l1BuildStep4,
	}
)

// Deploy verse builder contracts.
func Deploy(state *state.StateDB, block uint64) error {
	if block != 1 {
		return nil
	}
	for _, contract := range l1BuilderContracts {
		if err := contract.Deploy(state); err != nil {
			return err
		}
	}
	return nil
}
