package oasys

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

var (
	nftBridgeMainchain = &contract{
		name:         "NFTBridgeMainchain",
		address:      "0x520000000000000000000000000000000000000e",
		code:         nftBridgeMainchainCode,
		fixedStorage: map[string]interface{}{},
	}
	nftBridgeSidechain = &contract{
		name:         "NFTBridgeSidechain",
		address:      "0x520000000000000000000000000000000000000f",
		code:         nftBridgeSidechainCode,
		fixedStorage: map[string]interface{}{},
	}
	nftBridgeRelayer = &contract{
		name:    "NFTBridgeRelayer",
		address: "0x5200000000000000000000000000000000000010",
		code:    nftBridgeRelayerCode,
		fixedStorage: map[string]interface{}{
			// address public mainchainBridge
			"0x02": nftBridgeMainchain.address,
			// address public sidechainBridge
			"0x03": nftBridgeSidechain.address,
		},
	}

	nftBridgeContractSet = &nftbridge{
		nftBridgeMainchain,
		nftBridgeSidechain,
		nftBridgeRelayer,
	}
)

type nftbridge contractSet

func (p *nftbridge) deploy(state StateDB) {
	var owner common.Address

	switch GenesisHash {
	case params.OasysMainnetGenesisHash:
		owner = common.HexToAddress(mainnetGenesisWallet)
	case params.OasysTestnetGenesisHash:
		owner = common.HexToAddress(testnetGenesisWallet)
	}

	for _, c := range *p {
		cpy := c.copy()
		if (c == nftBridgeMainchain || c == nftBridgeSidechain) && owner != (common.Address{}) {
			// address private _owner
			cpy.fixedStorage["0x00"] = owner
		}
		cpy.deploy(state)
	}
}
