package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// EnvironmentValue is a representation of `Environment.EnvironmentValue`.
type EnvironmentValue struct {
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

// Determine if the given block number is the start block of the corresponding epoch.
func (p *EnvironmentValue) IsEpoch(number uint64) bool {
	return (number-p.StartBlock.Uint64())%p.EpochPeriod.Uint64() == 0
}

// Calculate epoch number from the given block number.
func (p *EnvironmentValue) Epoch(number uint64) uint64 {
	return p.StartEpoch.Uint64() + (number-p.StartBlock.Uint64())/p.EpochPeriod.Uint64()
}

// Determine if the given block number is the start block of the corresponding epoch.
func (p *EnvironmentValue) GetFirstBlock(number uint64) uint64 {
	elapsedEpoch := p.Epoch(number) - p.StartEpoch.Uint64()
	return p.StartBlock.Uint64() + elapsedEpoch*p.EpochPeriod.Uint64()
}

// Calculate the block number where the next environment should start based on this environment.
func (p *EnvironmentValue) NewValueStartBlock(newValueStartEpoch uint64) uint64 {
	return p.StartBlock.Uint64() +
		(newValueStartEpoch-p.StartEpoch.Uint64())*p.EpochPeriod.Uint64()
}

// Safely copy all values and return a new pointer.
func (p *EnvironmentValue) Copy() *EnvironmentValue {
	return &EnvironmentValue{
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

// Returns the environment value in Genesis.
func InitialEnvironmentValue(cfg *ChainConfig) *EnvironmentValue {
	return &EnvironmentValue{
		StartBlock:         common.Big0,
		StartEpoch:         common.Big1,
		BlockPeriod:        big.NewInt(int64(cfg.Oasys.Period)),
		EpochPeriod:        big.NewInt(int64(cfg.Oasys.Epoch)),
		RewardRate:         big.NewInt(10),
		CommissionRate:     big.NewInt(10),
		ValidatorThreshold: new(big.Int).Mul(big.NewInt(Ether), big.NewInt(10_000_000)),
		JailThreshold:      big.NewInt(500),
		JailPeriod:         big.NewInt(2),
	}
}

// Return the environment value updated in Sep 2024.
func ShortenedBlockTimeEnvironmentValue(cfg *ChainConfig) *EnvironmentValue {
	prev := InitialEnvironmentValue(cfg)
	next := prev.Copy()

	next.StartEpoch = cfg.OasysShortenedBlockTimeStartEpoch()
	next.StartBlock = new(big.Int).SetUint64(prev.NewValueStartBlock(next.StartEpoch.Uint64()))

	if cfg.ChainID.Cmp(OasysMainnetChainConfig.ChainID) == 0 ||
		cfg.ChainID.Cmp(OasysTestnetChainConfig.ChainID) == 0 {
		next.BlockPeriod = big.NewInt(6)
		next.EpochPeriod = big.NewInt(14400)
	}
	return next
}
