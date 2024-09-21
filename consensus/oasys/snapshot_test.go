package oasys

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalValidatorInfo(t *testing.T) {
	jsonbody := []byte(`[
		1,
		{
			"stake": 2,
			"index": 3,
			"vote_address": "0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004"
		}
	]`)

	expects := []*ValidatorInfo{
		&ValidatorInfo{
			Stake:       big.NewInt(1),
			Index:       0,
			VoteAddress: types.BLSPublicKey{},
		},
		&ValidatorInfo{
			Stake: big.NewInt(2),
			Index: 3,
			VoteAddress: types.BLSPublicKey([]byte{
				0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x4,
			}),
		},
	}

	var actuals []*ValidatorInfo
	err := json.Unmarshal(jsonbody, &actuals)
	require.NoError(t, err)
	require.Equal(t, expects, actuals)
}
