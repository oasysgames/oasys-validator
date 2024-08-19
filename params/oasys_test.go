package params

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestEnvironmentValue(t *testing.T) {
	env := &EnvironmentValue{
		StartBlock:         common.Big0,
		StartEpoch:         common.Big1,
		BlockPeriod:        big.NewInt(15),
		EpochPeriod:        big.NewInt(5760),
		RewardRate:         big.NewInt(10),
		CommissionRate:     big.NewInt(10),
		ValidatorThreshold: new(big.Int).Mul(big.NewInt(Ether), big.NewInt(10_000_000)),
		JailThreshold:      big.NewInt(500),
		JailPeriod:         big.NewInt(2),
	}

	newVal := env.Copy()
	newVal.StartEpoch = big.NewInt(3)
	newVal.StartBlock = new(big.Int).SetUint64(env.NewValueStartBlock(newVal.StartEpoch.Uint64()))

	// epoch 1
	for i, number := range []uint64{0, 1, 2, 5757, 5758, 5759} {
		if got := env.IsEpoch(number); (i == 0 && !got) || (i != 0 && got) {
			t.Errorf("IsEpoch(%d): should be %v", number, !got)
		}
		if got := env.Epoch(number); got != 1 {
			t.Errorf("Epoch(%d): want=1 got=%d", number, got)
		}
		if got := env.GetFirstBlock(number); got != 0 {
			t.Errorf("GetFirstBlock(%d): want=0 got=%d", number, got)
		}
		if got := env.NewValueStartBlock(newVal.StartEpoch.Uint64()); got != 11520 {
			t.Errorf("NewValueStartBlock(%d): want=11520 got=%d", number, got)
		}
	}
	// epoch 2
	for i, number := range []uint64{5760, 5761, 5762, 11517, 11518, 11519} {
		if got := env.IsEpoch(number); (i == 0 && !got) || (i != 0 && got) {
			t.Errorf("IsEpoch(%d): should be %v", number, !got)
		}
		if got := env.Epoch(number); got != 2 {
			t.Errorf("Epoch(%d): want=2 got=%d", number, got)
		}
		if got := env.GetFirstBlock(number); got != 5760 {
			t.Errorf("GetFirstBlock(%d): want=5760 got=%d", number, got)
		}
		if got := env.NewValueStartBlock(newVal.StartEpoch.Uint64()); got != 11520 {
			t.Errorf("NewValueStartBlock(%d): want=11520 got=%d", number, got)
		}
	}
	// epoch 3
	for i, number := range []uint64{11520, 11521, 11522, 17277, 17278, 17279} {
		if got := env.IsEpoch(number); (i == 0 && !got) || (i != 0 && got) {
			t.Errorf("IsEpoch(%d): should be %v", number, !got)
		}
		if got := env.Epoch(number); got != 3 {
			t.Errorf("Epoch(%d): want=3 got=%d", number, got)
		}
		if got := env.GetFirstBlock(number); got != 11520 {
			t.Errorf("GetFirstBlock(%d): want=11520 got=%d", number, got)
		}
		if got := env.NewValueStartBlock(newVal.StartEpoch.Uint64()); got != 11520 {
			t.Errorf("NewValueStartBlock(%d): want=11520 got=%d", number, got)
		}

		if got := newVal.IsEpoch(number); (i == 0 && !got) || (i != 0 && got) {
			t.Errorf("IsEpoch(%d): should be %v", number, !got)
		}
		if got := newVal.Epoch(number); got != 3 {
			t.Errorf("Epoch(%d): want=3 got=%d", number, got)
		}
		if got := newVal.GetFirstBlock(number); got != 11520 {
			t.Errorf("GetFirstBlock(%d): want=11520 got=%d", number, got)
		}
	}
}

func TestNewEnvironmentValue(t *testing.T) {
	compare := func(got, want *EnvironmentValue) {
		if got.StartBlock.Cmp(want.StartBlock) != 0 {
			t.Errorf("StartBlock: want=%d got=%d", want.StartBlock, got.StartBlock)
		}
		if got.StartEpoch.Cmp(want.StartEpoch) != 0 {
			t.Errorf("StartEpoch: want=%d got=%d", want.StartEpoch, got.StartEpoch)
		}
		if got.BlockPeriod.Cmp(want.BlockPeriod) != 0 {
			t.Errorf("BlockPeriod: want=%d got=%d", want.BlockPeriod, got.BlockPeriod)
		}
		if got.EpochPeriod.Cmp(want.EpochPeriod) != 0 {
			t.Errorf("EpochPeriod: want=%d got=%d", want.EpochPeriod, got.EpochPeriod)
		}
		if got.RewardRate.Cmp(want.RewardRate) != 0 {
			t.Errorf("RewardRate: want=%d got=%d", want.RewardRate, got.RewardRate)
		}
		if got.CommissionRate.Cmp(want.CommissionRate) != 0 {
			t.Errorf("CommissionRate: want=%d got=%d", want.CommissionRate, got.CommissionRate)
		}
		if got.ValidatorThreshold.Cmp(want.ValidatorThreshold) != 0 {
			t.Errorf("ValidatorThreshold: want=%d got=%d", want.ValidatorThreshold, got.ValidatorThreshold)
		}
		if got.JailThreshold.Cmp(want.JailThreshold) != 0 {
			t.Errorf("JailThreshold: want=%d got=%d", want.JailThreshold, got.JailThreshold)
		}
		if got.JailPeriod.Cmp(want.JailPeriod) != 0 {
			t.Errorf("JailPeriod: want=%d got=%d", want.JailPeriod, got.JailPeriod)
		}
	}

	compare(InitialEnvironmentValue(OasysMainnetChainConfig), &EnvironmentValue{
		StartBlock:         common.Big0,
		StartEpoch:         common.Big1,
		BlockPeriod:        big.NewInt(15),
		EpochPeriod:        big.NewInt(5760),
		RewardRate:         big.NewInt(10),
		CommissionRate:     big.NewInt(10),
		ValidatorThreshold: new(big.Int).Mul(big.NewInt(Ether), big.NewInt(10_000_000)),
		JailThreshold:      big.NewInt(500),
		JailPeriod:         big.NewInt(2),
	})

	compare(ShortenedBlockTimeEnvironmentValue(OasysMainnetChainConfig), &EnvironmentValue{
		StartBlock:         big.NewInt(5748480),
		StartEpoch:         big.NewInt(999),
		BlockPeriod:        big.NewInt(6),
		EpochPeriod:        big.NewInt(14400),
		RewardRate:         big.NewInt(10),
		CommissionRate:     big.NewInt(10),
		ValidatorThreshold: new(big.Int).Mul(big.NewInt(Ether), big.NewInt(10_000_000)),
		JailThreshold:      big.NewInt(500),
		JailPeriod:         big.NewInt(2),
	})
}
