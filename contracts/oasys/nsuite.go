package oasys

// N Suite multisig wallet / Powered by double jump.tokyo Inc.
var (
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
)
