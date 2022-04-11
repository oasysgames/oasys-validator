package oasys

var (
	tokenContractSet = contractSet{
		{
			name:    "L1StandardERC20Factory",
			address: Token.GetAddress(4096 * 1),
			code:    l1StandardERC20FactoryCode,
		},
		{
			name:    "L1StandardERC721Factory",
			address: Token.GetAddress(4096 * 2),
			code:    l1StandardERC721FactoryCode,
		},
	}
)
