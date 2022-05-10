package oasys

var (
	verifierInfo = &contract{
		name:    "VerifierInfo",
		address: "0x5200000000000000000000000000000000000003",
		code:    verifierInfoCode,
	}
	rollupContractSet = contractSet{
		verifierInfo,
	}
)
