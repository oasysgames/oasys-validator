package oasys

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie/testutil"
	"github.com/stretchr/testify/require"
)

func TestAssembleEnvAndValidators(t *testing.T) {
	var (
		header     = &types.Header{Extra: make([]byte, extraVanity)}
		config     = &params.OasysConfig{Period: 15, Epoch: 5760}
		env        = getInitialEnvironment(config)
		size       = 20
		validators = &nextValidators{
			Owners:        make([]common.Address, size),
			Operators:     make([]common.Address, size),
			Stakes:        make([]*big.Int, size),
			Indexes:       make([]int, size),
			VoteAddresses: make([]types.BLSPublicKey, size),
		}
	)

	for i := 0; i < size; i++ {
		validators.Owners[i] = testutil.RandomAddress()
		validators.Operators[i] = testutil.RandomAddress()
		validators.Stakes[i] = newEth(int64(i))
		validators.Indexes[i] = i
		validators.VoteAddresses[i] = randomBLSPublicKey()
	}

	// assemble
	header.Extra = append(header.Extra, assembleEnvironmentValue(env)...)
	require.Len(t, header.Extra, extraVanity+envValuesLen)
	header.Extra = append(header.Extra, assembleValidators(validators)...)
	require.Len(t, header.Extra, extraVanity+envValuesLen+validatorNumberSize+size*validatorInfoBytesLen)

	// disassemble
	acturalEnv, err := getEnvironmentFromHeader(header)
	require.NoError(t, err)
	actualVals, err := getValidatorsFromHeader(header)
	require.NoError(t, err)

	// assert env
	require.ElementsMatch(t, env.StartBlock.Bytes(), acturalEnv.StartBlock.Bytes())
	require.ElementsMatch(t, env.StartEpoch.Bytes(), acturalEnv.StartEpoch.Bytes())
	require.ElementsMatch(t, env.BlockPeriod.Bytes(), acturalEnv.BlockPeriod.Bytes())
	require.ElementsMatch(t, env.EpochPeriod.Bytes(), acturalEnv.EpochPeriod.Bytes())
	require.ElementsMatch(t, env.RewardRate.Bytes(), acturalEnv.RewardRate.Bytes())
	require.ElementsMatch(t, env.CommissionRate.Bytes(), acturalEnv.CommissionRate.Bytes())
	require.ElementsMatch(t, env.ValidatorThreshold.Bytes(), acturalEnv.ValidatorThreshold.Bytes())
	require.ElementsMatch(t, env.JailThreshold.Bytes(), acturalEnv.JailThreshold.Bytes())
	require.ElementsMatch(t, env.JailPeriod.Bytes(), acturalEnv.JailPeriod.Bytes())
	// assert validators
	for i := 0; i < size; i++ {
		// require.ElementsMatch(t, validators.Owners[i])
		require.ElementsMatch(t, validators.Operators[i].Bytes(), actualVals.Operators[i].Bytes())
		require.ElementsMatch(t, validators.Owners[i].Bytes(), actualVals.Owners[i].Bytes())
		require.ElementsMatch(t, validators.Stakes[i].Bytes(), actualVals.Stakes[i].Bytes())
		require.Equal(t, validators.Indexes[i], actualVals.Indexes[i])
		require.ElementsMatch(t, validators.VoteAddresses[i].Bytes(), actualVals.VoteAddresses[i].Bytes())
	}
}

func newEth(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), big.NewInt(params.Ether))
}

func randomBLSPublicKey() types.BLSPublicKey {
	return types.BLSPublicKey(testutil.RandBytes(types.BLSPublicKeyLength))
}
