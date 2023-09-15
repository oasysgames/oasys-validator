package oasys

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

var (
	epochPeriod = big.NewInt(40)

	validators = []common.Address{
		common.HexToAddress("0xd0887E868eCd4b16B75c60595DD0a7bA21Dbc0E9"),
		common.HexToAddress("0xEFb5BCC9b58fDaBEc50d3bFb5838f8FBf65B5Bba"),
		common.HexToAddress("0xd96FF3bfe34AF3680806d25cbfc1e138924f0Af2"),
		common.HexToAddress("0xa0BE9Fa71e130df9B033F24AF4b8cD944976081a"),
	}
	stakes = []*big.Int{
		new(big.Int).Mul(big.NewInt(10_000_000), ether),
		new(big.Int).Mul(big.NewInt(10_000_000), ether),
		new(big.Int).Mul(big.NewInt(10_000_000), ether),
		new(big.Int).Mul(big.NewInt(10_000_000), ether),
	}
	names = map[common.Address]string{
		validators[0]: "validator-0",
		validators[1]: "validator-1",
		validators[2]: "validator-2",
		validators[3]: "validator-3",
	}
	wantSchedules = []struct {
		block uint64
		turns []int
	}{
		// epoch 1
		{40, []int{1, 3, 2, 0}},
		{41, []int{0, 3, 1, 2}},
		{42, []int{2, 3, 0, 1}},
		{43, []int{1, 3, 2, 0}},
		{44, []int{0, 3, 1, 2}},
		{45, []int{0, 3, 1, 2}},
		{46, []int{1, 3, 0, 2}},
		{47, []int{0, 2, 3, 1}},
		{48, []int{0, 2, 3, 1}},
		{49, []int{1, 2, 3, 0}},
		{50, []int{0, 1, 2, 3}},
		{51, []int{2, 0, 1, 3}},
		{52, []int{2, 0, 1, 3}},
		{53, []int{2, 0, 1, 3}},
		{54, []int{1, 3, 0, 2}},
		{55, []int{0, 3, 2, 1}},
		{56, []int{1, 3, 2, 0}},
		{57, []int{0, 3, 2, 1}},
		{58, []int{0, 3, 2, 1}},
		{59, []int{1, 3, 2, 0}},
		{60, []int{1, 3, 2, 0}},
		{61, []int{1, 3, 2, 0}},
		{62, []int{0, 3, 2, 1}},
		{63, []int{2, 3, 1, 0}},
		{64, []int{1, 3, 0, 2}},
		{65, []int{0, 2, 3, 1}},
		{66, []int{0, 2, 3, 1}},
		{67, []int{3, 1, 2, 0}},
		{68, []int{2, 0, 1, 3}},
		{69, []int{2, 0, 1, 3}},
		{70, []int{1, 3, 0, 2}},
		{71, []int{0, 3, 1, 2}},
		{72, []int{3, 2, 0, 1}},
		{73, []int{3, 2, 1, 0}},
		{74, []int{3, 2, 0, 1}},
		{75, []int{3, 2, 1, 0}},
		{76, []int{3, 1, 0, 2}},
		{77, []int{3, 0, 2, 1}},
		{78, []int{3, 0, 2, 1}},
		{79, []int{3, 2, 1, 0}},
		// epoch 2
		{80, []int{0, 3, 1, 2}},
		{81, []int{0, 3, 1, 2}},
		{82, []int{1, 3, 0, 2}},
		{83, []int{1, 3, 0, 2}},
		{84, []int{0, 3, 1, 2}},
		{85, []int{2, 3, 0, 1}},
		{86, []int{2, 3, 1, 0}},
		{87, []int{2, 3, 0, 1}},
		{88, []int{1, 3, 2, 0}},
		{89, []int{0, 2, 1, 3}},
		{90, []int{2, 1, 0, 3}},
		{91, []int{1, 0, 3, 2}},
		{92, []int{0, 1, 3, 2}},
		{93, []int{1, 0, 3, 2}},
		{94, []int{0, 3, 2, 1}},
		{95, []int{0, 3, 2, 1}},
		{96, []int{3, 2, 1, 0}},
		{97, []int{3, 2, 0, 1}},
		{98, []int{3, 2, 0, 1}},
		{99, []int{3, 2, 0, 1}},
		{100, []int{3, 1, 2, 0}},
		{101, []int{2, 0, 1, 3}},
		{102, []int{2, 0, 1, 3}},
		{103, []int{1, 2, 0, 3}},
		{104, []int{0, 2, 1, 3}},
		{105, []int{3, 1, 0, 2}},
		{106, []int{3, 0, 1, 2}},
		{107, []int{3, 0, 1, 2}},
		{108, []int{3, 1, 0, 2}},
		{109, []int{3, 1, 0, 2}},
		{110, []int{2, 0, 3, 1}},
		{111, []int{2, 0, 3, 1}},
		{112, []int{1, 3, 2, 0}},
		{113, []int{0, 2, 1, 3}},
		{114, []int{3, 1, 0, 2}},
		{115, []int{3, 1, 0, 2}},
		{116, []int{2, 0, 3, 1}},
		{117, []int{1, 2, 3, 0}},
		{118, []int{0, 2, 3, 1}},
		{119, []int{1, 2, 3, 0}},
	}
)

func TestBackOffTimes(t *testing.T) {
	env := &environmentValue{
		StartBlock:  common.Big0,
		StartEpoch:  common.Big1,
		EpochPeriod: epochPeriod,
	}

	for _, s := range wantSchedules {
		chooser := newWeightedChooser(validators, stakes, int64(env.GetFirstBlock(s.block)))
		scheduler := newScheduler(env, env.GetFirstBlock(s.block), chooser)
		for i, validator := range validators {
			want := uint64(s.turns[i])
			if want > 0 {
				want += backoffWiggleTime
			}

			got := scheduler.backOffTime(s.block, validator)
			if got != want {
				t.Errorf("backoff mismatch, block %v, validator %v, got %v, want %v", s.block, names[validator], got, want)
			}
		}
	}
}

func TestExpect(t *testing.T) {
	env := &environmentValue{
		StartBlock:  common.Big0,
		StartEpoch:  common.Big1,
		EpochPeriod: epochPeriod,
	}

	for _, s := range wantSchedules {
		chooser := newWeightedChooser(validators, stakes, int64(env.GetFirstBlock(s.block)))
		scheduler := newScheduler(env, env.GetFirstBlock(s.block), chooser)

		var want common.Address
		for i, validator := range validators {
			if s.turns[i] == 0 {
				want = validator
			}
		}

		got := *scheduler.expect(s.block)
		if got != want {
			t.Errorf("schedule mismatch, block %v, got %v, want %v", s.block, names[got], names[want])
		}
	}
}

func TestDifficulty(t *testing.T) {
	env := &environmentValue{
		StartBlock:         common.Big0,
		StartEpoch:         common.Big1,
		EpochPeriod:        epochPeriod,
		ValidatorThreshold: new(big.Int).Mul(ether, big.NewInt(10_000_000)),
	}

	for _, s := range wantSchedules {
		chooser := newWeightedChooser(validators, stakes, int64(env.GetFirstBlock(s.block)))
		scheduler := newScheduler(env, env.GetFirstBlock(s.block), chooser)

		for i, validator := range validators {
			want1 := diffNoTurn.Uint64()
			if s.turns[i] == 0 {
				want1 = diffInTurn.Uint64()
			}
			want2 := uint64(1000) * uint64(len(validators)-s.turns[i])

			got1 := scheduler.difficulty(s.block, validator, false).Uint64()
			got2 := scheduler.difficulty(s.block, validator, true).Uint64()
			if got1 != want1 {
				t.Errorf("difficulty mismatch, block %v, validator %v, got %v, want %v", s.block, names[validator], got1, want1)
			}
			if got2 != want2 {
				t.Errorf("difficulty mismatch, block %v, validator %v, got %v, want %v", s.block, names[validator], got2, want2)
			}
		}
	}
}
