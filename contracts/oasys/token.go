package oasys

import "math/big"

var (
	tokenContractSet = contractSet{
		{
			name:    "WOAS",
			address: "0x5200000000000000000000000000000000000001",
			code:    wrappedOASCode,
			fixedStorage: map[string]interface{}{
				// string public name
				"0x00": "Wrapped OAS",
				// string public symbol
				"0x01": "WOAS",
				// uint8 public decimals
				"0x02": big.NewInt(18),
			},
		},
		{
			name:    "SOAS",
			address: "0x5200000000000000000000000000000000000002",
			code:    stakableOASCode,
			fixedStorage: map[string]interface{}{
				// string public name
				"0x03": "Stakable OAS",
				// string public symbol
				"0x04": "SOAS",
				// address public staking
				"0x05": "0x0000000000000000000000000000000000001001",
			},
		},
	}
)
