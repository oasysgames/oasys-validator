package oasys

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

// environmentValue is a representation of `Environment.EnvironmentValue`.
// https://github.com/oasysgames/oasys-genesis-contract/blob/3b87148/contracts/IEnvironment.sol#L13-L31
type environmentValue struct {
	// Block and epoch to which this setting applies
	StartBlock *big.Int
	StartEpoch *big.Int
	// Block generation interval(by seconds)
	BlockPeriod *big.Int
	// Number of blocks in epoch
	EpochPeriod *big.Int
	// Annual rate of staking reward
	RewardRate *big.Int
	// Validator commission rate
	CommissionRate *big.Int
	// Amount of tokens required to become a validator
	ValidatorThreshold *big.Int
	// Number of not sealed to jailing the validator
	JailThreshold *big.Int
	// Number of epochs to jailing the validator
	JailPeriod *big.Int
}

// Safely copy all values and return a new pointer.
func (p *environmentValue) Copy() *environmentValue {
	return &environmentValue{
		StartBlock:         new(big.Int).Set(p.StartBlock),
		StartEpoch:         new(big.Int).Set(p.StartEpoch),
		BlockPeriod:        new(big.Int).Set(p.BlockPeriod),
		EpochPeriod:        new(big.Int).Set(p.EpochPeriod),
		RewardRate:         new(big.Int).Set(p.RewardRate),
		CommissionRate:     new(big.Int).Set(p.CommissionRate),
		ValidatorThreshold: new(big.Int).Set(p.ValidatorThreshold),
		JailThreshold:      new(big.Int).Set(p.JailThreshold),
		JailPeriod:         new(big.Int).Set(p.JailPeriod),
	}
}

// Check if all values are equal to `other`.
func (p *environmentValue) Equals(other *environmentValue) bool {
	comps := []int{
		p.StartBlock.Cmp(other.StartBlock),
		p.StartEpoch.Cmp(other.StartEpoch),
		p.BlockPeriod.Cmp(other.BlockPeriod),
		p.EpochPeriod.Cmp(other.EpochPeriod),
		p.RewardRate.Cmp(other.RewardRate),
		p.CommissionRate.Cmp(other.CommissionRate),
		p.ValidatorThreshold.Cmp(other.ValidatorThreshold),
		p.JailThreshold.Cmp(other.JailThreshold),
		p.JailPeriod.Cmp(other.JailPeriod),
	}
	for _, c := range comps {
		if c != 0 {
			return false
		}
	}
	return true
}

// Calculate epoch number from the given block number.
func (p *environmentValue) Epoch(number uint64) uint64 {
	return p.StartEpoch.Uint64() + (number-p.StartBlock.Uint64())/p.EpochPeriod.Uint64()
}

// Calculate the block number where the next environment should start based on this environment.
func (p *environmentValue) NewValueStartBlock(newValue *environmentValue) uint64 {
	return p.StartBlock.Uint64() +
		(newValue.StartEpoch.Uint64()-p.StartEpoch.Uint64())*p.EpochPeriod.Uint64()
}

// Returns whether this environment can be applied to the given block number.
func (p *environmentValue) Started(number uint64) bool {
	return number >= p.StartBlock.Uint64()
}

// Determine if the given block number is the start block of the corresponding epoch.
func (p *environmentValue) IsEpochStartBlock(number uint64) bool {
	return (number-p.StartBlock.Uint64())%p.EpochPeriod.Uint64() == 0
}

// Calculate the start block number of the corresponding epoch from the given block number.
func (p *environmentValue) EpochStartBlock(number uint64) uint64 {
	elapsedEpoch := p.Epoch(number) - p.StartEpoch.Uint64()
	return p.StartBlock.Uint64() + elapsedEpoch*p.EpochPeriod.Uint64()
}

// Determine if the given block number is suitable for deploying a new environment.
func (p *environmentValue) ShouldUpdate(newValue *environmentValue, number uint64) bool {
	if p.Epoch(number)-1 != newValue.StartEpoch.Uint64() {
		// past or future epoch
		return false
	}
	// check if it is in the middle of the epoch
	return (number - p.EpochStartBlock(number)) == p.EpochPeriod.Uint64()/2
}

// Returns the environment value to be used under the given block
// and the environment value that is pending application next.
// The value returned by `next` is the environment value that should be deployed
// via the `Environment.updateValue(...)` contract method. It is not the environment
// value that should be applied in the next block or epoch.
var getEnvironmentValue = func() func(cfg *params.ChainConfig, number uint64) (curr, next *environmentValue) {
	updates := []func(cfg *params.ChainConfig, env *environmentValue){
		// Genesis
		func(cfg *params.ChainConfig, env *environmentValue) {
			env.StartBlock = common.Big0
			env.StartEpoch = common.Big1
			env.BlockPeriod = big.NewInt(int64(cfg.Oasys.Period))
			env.EpochPeriod = big.NewInt(int64(cfg.Oasys.Epoch))
			env.RewardRate = big.NewInt(10)
			env.CommissionRate = big.NewInt(10)
			env.ValidatorThreshold = new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(10_000_000))
			env.JailThreshold = big.NewInt(500)
			env.JailPeriod = big.NewInt(2)
		},

		// Sep 2024
		func(cfg *params.ChainConfig, env *environmentValue) {
			env.StartEpoch = cfg.OasysShortenedBlockTimeStartEpoch()

			if cfg.ChainID.Cmp(params.OasysMainnetChainConfig.ChainID) == 0 ||
				cfg.ChainID.Cmp(params.OasysTestnetChainConfig.ChainID) == 0 {
				env.BlockPeriod = big.NewInt(6)
				env.EpochPeriod = big.NewInt(14400)
			}
		},
	}

	var (
		mu    sync.Mutex
		cache []*environmentValue
	)
	return func(cfg *params.ChainConfig, number uint64) (curr *environmentValue, next *environmentValue) {
		mu.Lock()
		defer mu.Unlock()

		if len(cache) > 0 {
			curr = cache[0]
		} else {
			curr = &environmentValue{}
			cache = append(cache, curr)
			updates[0](cfg, curr)
		}

		for i, update := range updates[1:] {
			if len(cache) > i+1 {
				next = cache[i+1]
			} else {
				next = curr.Copy()
				cache = append(cache, next)

				update(cfg, next)
				next.StartBlock = new(big.Int).SetUint64(curr.NewValueStartBlock(next))
			}

			if !next.Started(number) {
				break
			}
			curr = next
		}

		if next == nil {
			next = curr.Copy()
		}
		return curr, next
	}
}()
