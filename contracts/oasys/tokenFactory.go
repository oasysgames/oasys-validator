package oasys

var (
	tokenFactoryContractSet = contractSet{
		{
			name:    "L1StandardERC20Factory",
			address: "0x5200000000000000000000000000000000000004",
			code:    l1StandardERC20FactoryCode,
		},
		{
			name:    "L1StandardERC721Factory",
			address: "0x5200000000000000000000000000000000000005",
			code:    l1StandardERC721FactoryCode,
		},
	}
)
