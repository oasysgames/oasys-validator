package oasys

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

func TestEnvironmentValue(t *testing.T) {
	env := &environmentValue{
		StartBlock:         common.Big0,
		StartEpoch:         common.Big1,
		BlockPeriod:        big.NewInt(15),
		EpochPeriod:        big.NewInt(5760),
		RewardRate:         big.NewInt(10),
		CommissionRate:     big.NewInt(10),
		ValidatorThreshold: new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(10_000_000)),
		JailThreshold:      big.NewInt(500),
		JailPeriod:         big.NewInt(2),
	}

	newVal := env.Copy()
	newVal.StartEpoch = big.NewInt(3)
	newVal.StartBlock = new(big.Int).SetUint64(env.NewValueStartBlock(newVal))

	if env.Equals(env) != true {
		t.Error("Equals(): should be true")
	}
	if env.Equals(newVal) != false {
		t.Error("Equals(): should be false")
	}

	// epoch 1
	for i, number := range []uint64{0, 1, 2, 5757, 5758, 5759} {
		if got := env.Epoch(number); got != 1 {
			t.Errorf("Epoch(%d): want=1 got=%d", number, got)
		}
		if got := env.NewValueStartBlock(newVal); got != 11520 {
			t.Errorf("NewValueStartBlock(%d): want=11520 got=%d", number, got)
		}
		if got := env.Started(number); !got {
			t.Errorf("Started(%d): should be true", number)
		}
		if got := env.IsEpochStartBlock(number); (i == 0 && !got) || (i != 0 && got) {
			t.Errorf("IsEpochStartBlock(%d): should be %v", number, !got)
		}
		if got := env.EpochStartBlock(number); got != 0 {
			t.Errorf("EpochStartBlock(%d): want=0 got=%d", number, got)
		}
		if got := env.ShouldUpdate(newVal, number); got {
			t.Errorf("ShouldUpdate(newVal, %d): should be false", number)
		}

		if got := newVal.Started(number); got {
			t.Errorf("Started(%d): should be false", number)
		}
	}
	// epoch 2
	for i, number := range []uint64{5760, 5761, 5762, 11517, 11518, 11519} {
		if got := env.Epoch(number); got != 2 {
			t.Errorf("Epoch(%d): want=2 got=%d", number, got)
		}
		if got := env.NewValueStartBlock(newVal); got != 11520 {
			t.Errorf("NewValueStartBlock(%d): want=11520 got=%d", number, got)
		}
		if got := env.Started(number); !got {
			t.Errorf("Started(%d): should be true", number)
		}
		if got := env.IsEpochStartBlock(number); (i == 0 && !got) || (i != 0 && got) {
			t.Errorf("IsEpochStartBlock(%d): should be %v", number, !got)
		}
		if got := env.EpochStartBlock(number); got != 5760 {
			t.Errorf("EpochStartBlock(%d): want=5760 got=%d", number, got)
		}
		if got := env.ShouldUpdate(newVal, number); (number == 8640 && !got) || (number != 8640 && got) {
			t.Errorf("ShouldUpdate(newVal, %d): should be false", number)
		}

		if got := newVal.Started(number); got {
			t.Errorf("Started(%d): should be false", number)
		}
	}
	// epoch 3
	for i, number := range []uint64{11520, 11521, 11522, 17277, 17278, 17279} {
		if got := env.Epoch(number); got != 3 {
			t.Errorf("Epoch(%d): want=3 got=%d", number, got)
		}
		if got := env.NewValueStartBlock(newVal); got != 11520 {
			t.Errorf("NewValueStartBlock(%d): want=11520 got=%d", number, got)
		}
		if got := env.Started(number); !got {
			t.Errorf("Started(%d): should be true", number)
		}
		if got := env.IsEpochStartBlock(number); (i == 0 && !got) || (i != 0 && got) {
			t.Errorf("IsEpochStartBlock(%d): should be %v", number, !got)
		}
		if got := env.EpochStartBlock(number); got != 11520 {
			t.Errorf("EpochStartBlock(%d): want=11520 got=%d", number, got)
		}
		if got := env.ShouldUpdate(newVal, number); got {
			t.Errorf("ShouldUpdate(newVal, %d): should be false", number)
		}

		if got := newVal.Epoch(number); got != 3 {
			t.Errorf("Epoch(%d): want=3 got=%d", number, got)
		}
		if got := newVal.Started(number); !got {
			t.Errorf("Started(%d): should be true", number)
		}
		if got := newVal.IsEpochStartBlock(number); (i == 0 && !got) || (i != 0 && got) {
			t.Errorf("IsEpochStartBlock(%d): should be %v", number, !got)
		}
		if got := newVal.EpochStartBlock(number); got != 11520 {
			t.Errorf("EpochStartBlock(%d): want=11520 got=%d", number, got)
		}
	}
}

func TestGetEnvironmentValue(t *testing.T) {
	compare := func(number uint64, got, want *environmentValue) {
		if got.StartBlock.Cmp(want.StartBlock) != 0 {
			t.Errorf("StartBlock: number=%d want=%d got=%d",
				number, want.StartBlock, got.StartBlock)
		}
		if got.StartEpoch.Cmp(want.StartEpoch) != 0 {
			t.Errorf("StartEpoch: number=%d want=%d got=%d",
				number, want.StartEpoch, got.StartEpoch)
		}
		if got.BlockPeriod.Cmp(want.BlockPeriod) != 0 {
			t.Errorf("BlockPeriod: number=%d want=%d got=%d",
				number, want.BlockPeriod, got.BlockPeriod)
		}
		if got.EpochPeriod.Cmp(want.EpochPeriod) != 0 {
			t.Errorf("EpochPeriod: number=%d want=%d got=%d",
				number, want.EpochPeriod, got.EpochPeriod)
		}
		if got.RewardRate.Cmp(want.RewardRate) != 0 {
			t.Errorf("RewardRate: number=%d want=%d got=%d",
				number, want.RewardRate, got.RewardRate)
		}
		if got.CommissionRate.Cmp(want.CommissionRate) != 0 {
			t.Errorf("CommissionRate: number=%d want=%d got=%d",
				number, want.CommissionRate, got.CommissionRate)
		}
		if got.ValidatorThreshold.Cmp(want.ValidatorThreshold) != 0 {
			t.Errorf("ValidatorThreshold: number=%d want=%d got=%d",
				number, want.ValidatorThreshold, got.ValidatorThreshold)
		}
		if got.JailThreshold.Cmp(want.JailThreshold) != 0 {
			t.Errorf("JailThreshold: number=%d want=%d got=%d",
				number, want.JailThreshold, got.JailThreshold)
		}
		if got.JailPeriod.Cmp(want.JailPeriod) != 0 {
			t.Errorf("JailPeriod: number=%d want=%d got=%d",
				number, want.JailPeriod, got.JailPeriod)
		}
	}

	want0 := &environmentValue{
		StartBlock:         common.Big0,
		StartEpoch:         common.Big1,
		BlockPeriod:        big.NewInt(15),
		EpochPeriod:        big.NewInt(5760),
		RewardRate:         big.NewInt(10),
		CommissionRate:     big.NewInt(10),
		ValidatorThreshold: new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(10_000_000)),
		JailThreshold:      big.NewInt(500),
		JailPeriod:         big.NewInt(2),
	}
	want1 := &environmentValue{
		StartBlock:         big.NewInt(5748480),
		StartEpoch:         big.NewInt(999),
		BlockPeriod:        big.NewInt(6),
		EpochPeriod:        big.NewInt(14400),
		RewardRate:         big.NewInt(10),
		CommissionRate:     big.NewInt(10),
		ValidatorThreshold: new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(10_000_000)),
		JailThreshold:      big.NewInt(500),
		JailPeriod:         big.NewInt(2),
	}

	cases := []struct {
		numbers            []uint64
		wantCurr, wantNext *environmentValue
	}{
		// epoch 1
		{[]uint64{0, 1, 5758, 5759}, want0, want1},
		// epoch 2
		{[]uint64{5760, 5761, 11518, 11519}, want0, want1},
		// epoch 998
		{[]uint64{5742720, 5742721, 5748478, 5748479}, want0, want1},
		// epoch 999
		{[]uint64{5748480, 5748481, 5754238, 5754239}, want1, want1},
		// epoch 1000
		{[]uint64{5754240, 5754241, 5759998, 5759999}, want1, want1},
	}

	for _, c := range cases {
		for _, number := range c.numbers {
			gotCurr, gotNext := getEnvironmentValue(params.OasysMainnetChainConfig, number)
			compare(number, gotCurr, c.wantCurr)
			compare(number, gotNext, c.wantNext)
		}
	}
}
