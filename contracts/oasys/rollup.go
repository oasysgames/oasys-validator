package oasys

var (
	verifierInfo = &contract{
		name:    "VerifierInfo",
		address: Rollup.GetAddress(4096 * 1),
		code:    verifierInfoCode,
	}
	rollupContractSet = contractSet{
		verifierInfo,
	}
)
