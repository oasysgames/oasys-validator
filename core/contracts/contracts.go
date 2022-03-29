package contracts

type Contract struct {
	Address string
	Code    string
	ABI     string
}

var (
	// Oasys system contracts
	Environment  = Contract{"0x0000000000000000000000000000000000001000", environmentCode, environmentAbi}
	StakeManager = Contract{"0x0000000000000000000000000000000000001001", stakeManagerCode, stakeManagerAbi}
	Contracts    = []Contract{Environment, StakeManager}
)
