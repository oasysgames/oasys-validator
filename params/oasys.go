package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

const (
	SHORT_BLOCK_TIME_SECONDS      = 6
	SHORT_BLOCK_TIME_EPOCH_PERIOD = 14400 // 6 sec * 14400 block = 1 days

	SHORT_BLOCK_TIME_FORK_EPOCH_MAINNET = 711 // Block #4089600
	SHORT_BLOCK_TIME_FORK_EPOCH_TESTNET = 699 // Block #4020480
	SHORT_BLOCK_TIME_FORK_EPOCH_OTHERS  = 10  // for local chain
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
func InitialEnvironmentValue(cfg *OasysConfig) *EnvironmentValue {
	return &EnvironmentValue{
		StartBlock:         common.Big0,
		StartEpoch:         common.Big1,
		BlockPeriod:        new(big.Int).SetUint64(cfg.Period),
		EpochPeriod:        new(big.Int).SetUint64(cfg.Epoch),
		RewardRate:         big.NewInt(10),
		CommissionRate:     big.NewInt(10),
		ValidatorThreshold: new(big.Int).Mul(big.NewInt(Ether), big.NewInt(10_000_000)),
		JailThreshold:      big.NewInt(500),
		JailPeriod:         big.NewInt(2),
	}
}
