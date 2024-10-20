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

	// test `Equal` method
	wantErr := "mismatching start block, expected: 0, real: 11520"
	gotErr := newVal.Equal(env).Error()
	if gotErr != wantErr {
		t.Errorf("Equal(env): want=`%s` got=`%s`", wantErr, gotErr)
	}
}
