package oasys

var (
	verifierInfo = &contract{
		name:    "VerifierInfo",
		address: "0x5200000000000000000000000000000000000002",
		code:    verifierInfoCode,
	}
	rollupContractSet = contractSet{
		verifierInfo,
	}
)
