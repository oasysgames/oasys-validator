package oasys

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/params"
)

var (
	l1BuildParamAddress     = "0x5200000000000000000000000000000000000006"
	l1BuildDepositAddress   = "0x5200000000000000000000000000000000000007"
	l1BuildAgentAddress     = "0x5200000000000000000000000000000000000008"
	l1BuildStep1Address     = "0x5200000000000000000000000000000000000009"
	l1BuildStep2Address     = "0x520000000000000000000000000000000000000a"
	l1BuildStep3Address     = "0x520000000000000000000000000000000000000b"
	l1BuildStep4Address     = "0x520000000000000000000000000000000000000c"
	l1BuildAllowListAddress = "0x520000000000000000000000000000000000000d"

	l1BuildParam = &contract{
		name:    "L1BuildParam",
		address: l1BuildParamAddress,
		code:    l1BuildParamCode,
		fixedStorage: map[string]interface{}{
			// uint256 public maxTransactionGasLimit
			"0x00": big.NewInt(15_000_000),
			// uint256 public l2GasDiscountDivisor
			"0x01": big.NewInt(32),
			// uint256 public enqueueGasCost
			"0x02": big.NewInt(60_000),
			// uint256 public fraudProofWindow
			"0x03": big.NewInt(604_800),
			// uint256 public sequencerPublishWindow
			"0x04": big.NewInt(12_592_000),
		},
		dynamicStorage: map[string]string{
			// bytes public l1StandardBridgeCode
			"0x05": l1StandardBridgeCode,
			// bytes public l1ERC721BridgeCode
			"0x06": l1ERC721BridgeCode,
		},
	}
	l1BuildDeposit = &contract{
		name:    "L1BuildDeposit",
		address: l1BuildDepositAddress,
		code:    l1BuildDepositCode,
		fixedStorage: map[string]interface{}{
			// address public allowlistAddress
			"0x02": l1BuildAllowListAddress,
			// address public agentAddress
			"0x03": l1BuildAgentAddress,
		},
	}
	l1BuildAgent = &contract{
		name:    "L1BuildAgent",
		address: l1BuildAgentAddress,
		code:    l1BuildAgentCode,
		fixedStorage: map[string]interface{}{
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
	l1BuildStep1 = &contract{
		name:    "L1BuildStep1",
		address: l1BuildStep1Address,
		code:    l1BuildStep1Code,
		fixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
			// address public paramAddress
			"0x01": l1BuildParamAddress,
		},
	}
	l1BuildStep2 = &contract{
		name:    "L1BuildStep2",
		address: l1BuildStep2Address,
		code:    l1BuildStep2Code,
		fixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
			// address public paramAddress
			"0x01": l1BuildParamAddress,
			// address public verifierInfoAddress
			"0x02": verifierInfo.address,
		},
	}
	l1BuildStep3 = &contract{
		name:    "L1BuildStep3",
		address: l1BuildStep3Address,
		code:    l1BuildStep3Code,
		fixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
		},
	}
	l1BuildStep4 = &contract{
		name:    "L1BuildStep4",
		address: l1BuildStep4Address,
		code:    l1BuildStep4Code,
		fixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
			// address public paramAddress
			"0x01": l1BuildParamAddress,
		},
	}
	l1BuildAllowList = &contract{
		name:         "L1BuildAllowList",
		address:      l1BuildAllowListAddress,
		code:         allowListCode,
		fixedStorage: map[string]interface{}{},
	}

	verseBuilderContractSet = &versebuilder{
		l1BuildParam,
		l1BuildDeposit,
		l1BuildAgent,
		l1BuildStep1,
		l1BuildStep2,
		l1BuildStep3,
		l1BuildStep4,
		l1BuildAllowList,
	}
)

type versebuilder contractSet

func (p *versebuilder) deploy(state *state.StateDB) {
	switch GenesisHash {
	case params.OasysMainnetGenesisHash:
		// uint256 public requiredAmount
		l1BuildDeposit.fixedStorage["0x00"] = new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(1_000_000))
		// uint256 public lockedBlock
		l1BuildDeposit.fixedStorage["0x01"] = big.NewInt(1_036_800)
		// address private _owner
		l1BuildAllowList.fixedStorage["0x00"] = common.HexToAddress(mainnetGenesisWallet)
	default:
		l1BuildDeposit.fixedStorage["0x00"] = big.NewInt(params.GWei)
		l1BuildDeposit.fixedStorage["0x01"] = common.Big1
	}

	for _, c := range *p {
		c.deploy(state)
	}
}
