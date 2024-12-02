package oasys

var (
	// Genesis
	environment = &contract{
		name:    "Environment",
		address: BuiltInContractPrefix1 + "000000000000000000001000",
	}
	stakeManager = &contract{
		name:    "StakeManager",
		address: BuiltInContractPrefix1 + "000000000000000000001001",
	}
	oasMultiTransfer = &contract{
		name:    "OASMultiTransfer",
		address: BuiltInContractPrefix2 + "00000000000000000000002c",
	}
	candidateValidatorManagerHighStakes = &contract{
		name:    "CandidateValidatorManager/highStakes",
		address: BuiltInContractPrefix2 + "00000000000000000000002D",
	}
	candidateValidatorManager = &contract{
		name:    "CandidateValidatorManager",
		address: BuiltInContractPrefix2 + "00000000000000000000002e",
	}

	// Governance
	permissionedContractFactory = &contract{
		name:    "PermissionedContractFactory",
		address: BuiltInContractPrefix2 + "00000000000000000000002F",
	}
	evmAccessControl = &contract{
		name:    "EVMAccessControl",
		address: BuiltInContractPrefix2 + "00000000000000000000003F",
	}

	// ERC20 Tokens
	wrappedOAS = &contract{
		name:    "WOAS",
		address: BuiltInContractPrefix2 + "000000000000000000000001",
	}
	stakableOAS = &contract{
		name:    "SOAS",
		address: BuiltInContractPrefix2 + "000000000000000000000002",
	}
	lockedOAS = &contract{
		name:    "LOAS",
		address: BuiltInContractPrefix2 + "000000000000000000000023",
	}

	// FT/NFT Factory
	l1StandardERC20Factory = &contract{
		name:    "L1StandardERC20Factory",
		address: BuiltInContractPrefix2 + "000000000000000000000004",
	}
	l1StandardERC721Factory = &contract{
		name:    "L1StandardERC721Factory",
		address: BuiltInContractPrefix2 + "000000000000000000000005",
	}

	// Verse Builder
	l1BuildParam = &contract{
		name:    "L1BuildParam",
		address: BuiltInContractPrefix2 + "000000000000000000000006",
	}
	l1BuildDeposit = &contract{
		name:    "L1BuildDeposit",
		address: BuiltInContractPrefix2 + "000000000000000000000007",
	}
	l1BuildAgent = &contract{
		name:    "L1BuildAgent",
		address: BuiltInContractPrefix2 + "000000000000000000000008",
	}
	l1BuildStep1 = &contract{
		name:    "L1BuildStep1",
		address: BuiltInContractPrefix2 + "000000000000000000000009",
	}
	l1BuildStep2 = &contract{
		name:    "L1BuildStep2",
		address: BuiltInContractPrefix2 + "00000000000000000000000a",
	}
	l1BuildStep3 = &contract{
		name:    "L1BuildStep3",
		address: BuiltInContractPrefix2 + "00000000000000000000000b",
	}
	l1BuildStep4 = &contract{
		name:    "L1BuildStep4",
		address: BuiltInContractPrefix2 + "00000000000000000000000c",
	}
	l1BuildAllowList = &contract{
		name:    "L1BuildAllowList",
		address: BuiltInContractPrefix2 + "00000000000000000000000d",
	}

	// N Suite multisig wallet / Powered by double jump.tokyo Inc.
	nsuiteGnosisSafe = &contract{
		name:    "N Suite/GnosisSafe",
		address: BuiltInContractPrefix2 + "000000000000000000000011",
	}
	nsuiteGnosisSafeProxyFactory = &contract{
		name:    "N Suite/GnosisSafeProxyFactory",
		address: BuiltInContractPrefix2 + "000000000000000000000012",
	}
	nsuiteCompatibilityFallbackHandler = &contract{
		name:    "N Suite/CompatibilityFallbackHandler",
		address: BuiltInContractPrefix2 + "000000000000000000000013",
	}

	// Oasys State Commitment Chain Verifier
	sccVerifier = &contract{
		name:    "OasysStateCommitmentChainVerifier",
		address: BuiltInContractPrefix2 + "000000000000000000000014",
	}

	// cBridge / Powered by Celer Network.
	cBridgeBridge = &contract{
		name:    "cBridge/Bridge",
		address: BuiltInContractPrefix2 + "000000000000000000000015",
	}
	cBridgeOriginalTokenVaultV2 = &contract{
		name:    "cBridge/OriginalTokenVaultV2",
		address: BuiltInContractPrefix2 + "000000000000000000000016",
	}
	cBridgePeggedTokenBridgeV2 = &contract{
		name:    "cBridge/PeggedTokenBridgeV2",
		address: BuiltInContractPrefix2 + "000000000000000000000017",
	}

	// Tealswap / Powered by SOOHO.IO Inc.
	tealswapFactory = &contract{
		name:    "Tealswap/TealswapFactory",
		address: BuiltInContractPrefix2 + "000000000000000000000018",
	}
	tealswapRouter = &contract{
		name:    "Tealswap/TealswapRouter",
		address: BuiltInContractPrefix2 + "000000000000000000000019",
	}

	// tofuNFT / Powered by JINJA Foundation Ltd.
	tofunftMarketNG = &contract{
		name:    "tofuNFT/MarketNG",
		address: BuiltInContractPrefix2 + "000000000000000000000020",
	}

	// multicall
	multicall = &contract{
		name:    "Multicall",
		address: BuiltInContractPrefix2 + "000000000000000000000021",
	}
	multicall2 = &contract{
		name:    "Multicall2",
		address: BuiltInContractPrefix2 + "000000000000000000000022",
	}

	// bitbank
	bitbankExchangeDeposit = &contract{
		name:    "bitbank/ExchangeDeposit",
		address: BuiltInContractPrefix2 + "000000000000000000000024",
	}
	bitbankProxyFactory = &contract{
		name:    "bitbank/ProxyFactory",
		address: BuiltInContractPrefix2 + "000000000000000000000025",
	}
	bitbankExchangeDepositStaging = &contract{
		name:    "bitbank/ExchangeDeposit/Staging",
		address: BuiltInContractPrefix2 + "000000000000000000000026",
	}
	bitbankProxyFactoryStaging = &contract{
		name:    "bitbank/ProxyFactory/Staging",
		address: BuiltInContractPrefix2 + "000000000000000000000027",
	}
	bitbankExchangeDepositYokohama = &contract{
		name:    "bitbank/ExchangeDeposit/Yokohama",
		address: BuiltInContractPrefix2 + "000000000000000000000028",
	}
	bitbankProxyFactoryYokohama = &contract{
		name:    "bitbank/ProxyFactory/Yokohama",
		address: BuiltInContractPrefix2 + "000000000000000000000029",
	}
	bitbankExchangeDepositDev = &contract{
		name:    "bitbank/ExchangeDeposit/Dev",
		address: BuiltInContractPrefix2 + "00000000000000000000002A",
	}
	bitbankProxyFactoryDev = &contract{
		name:    "bitbank/ProxyFactory/Dev",
		address: BuiltInContractPrefix2 + "00000000000000000000002b",
	}
)

// Contract definition.
type contract struct {
	name    string
	address string
}
