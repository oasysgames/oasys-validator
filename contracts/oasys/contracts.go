package oasys

var (
	// ERC20 Tokens
	wrappedOAS = &contract{
		name:    "WOAS",
		address: "0x5200000000000000000000000000000000000001",
	}
	stakableOAS = &contract{
		name:    "SOAS",
		address: "0x5200000000000000000000000000000000000002",
	}

	// FT/NFT Factory
	l1StandardERC20Factory = &contract{
		name:    "L1StandardERC20Factory",
		address: "0x5200000000000000000000000000000000000004",
	}
	l1StandardERC721Factory = &contract{
		name:    "L1StandardERC721Factory",
		address: "0x5200000000000000000000000000000000000005",
	}

	// Verse Builder
	l1BuildParam = &contract{
		name:    "L1BuildParam",
		address: "0x5200000000000000000000000000000000000006",
	}
	l1BuildDeposit = &contract{
		name:    "L1BuildDeposit",
		address: "0x5200000000000000000000000000000000000007",
	}
	l1BuildAgent = &contract{
		name:    "L1BuildAgent",
		address: "0x5200000000000000000000000000000000000008",
	}
	l1BuildStep1 = &contract{
		name:    "L1BuildStep1",
		address: "0x5200000000000000000000000000000000000009",
	}
	l1BuildStep2 = &contract{
		name:    "L1BuildStep2",
		address: "0x520000000000000000000000000000000000000a",
	}
	l1BuildStep3 = &contract{
		name:    "L1BuildStep3",
		address: "0x520000000000000000000000000000000000000b",
	}
	l1BuildStep4 = &contract{
		name:    "L1BuildStep4",
		address: "0x520000000000000000000000000000000000000c",
	}
	l1BuildAllowList = &contract{
		name:    "L1BuildAllowList",
		address: "0x520000000000000000000000000000000000000d",
	}

	// N Suite multisig wallet / Powered by double jump.tokyo Inc.
	nsuiteGnosisSafe = &contract{
		name:    "N Suite/GnosisSafe",
		address: "0x5200000000000000000000000000000000000011",
	}
	nsuiteGnosisSafeProxyFactory = &contract{
		name:    "N Suite/GnosisSafeProxyFactory",
		address: "0x5200000000000000000000000000000000000012",
	}
	nsuiteCompatibilityFallbackHandler = &contract{
		name:    "N Suite/CompatibilityFallbackHandler",
		address: "0x5200000000000000000000000000000000000013",
	}

	// Oasys State Commitment Chain Verifier
	sccVerifier = &contract{
		name:    "OasysStateCommitmentChainVerifier",
		address: "0x5200000000000000000000000000000000000014",
	}
)

// Contract definition.
type contract struct {
	name    string
	address string
}
