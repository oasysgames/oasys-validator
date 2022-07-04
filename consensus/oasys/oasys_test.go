package oasys

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

var (
	epochPeriod = uint64(40)
	ether       = big.NewInt(1_000_000_000_000_000_000)

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
)

func TestAddBalanceToStakeManager(t *testing.T) {}

func TestBackOffTime(t *testing.T) {
	testCases := []struct {
		block uint64
		want  []uint64
	}{
		// epoch 1
		{40, []uint64{2, 4, 3, 0}},
		{41, []uint64{0, 4, 2, 3}},
		{42, []uint64{3, 4, 0, 2}},
		{43, []uint64{2, 4, 3, 0}},
		{44, []uint64{0, 4, 2, 3}},
		{45, []uint64{0, 4, 2, 3}},
		{46, []uint64{2, 4, 0, 3}},
		{47, []uint64{0, 3, 4, 2}},
		{48, []uint64{0, 3, 4, 2}},
		{49, []uint64{2, 3, 4, 0}},
		{50, []uint64{0, 2, 3, 4}},
		{51, []uint64{3, 0, 2, 4}},
		{52, []uint64{3, 0, 2, 4}},
		{53, []uint64{3, 0, 2, 4}},
		{54, []uint64{2, 4, 0, 3}},
		{55, []uint64{0, 4, 3, 2}},
		{56, []uint64{2, 4, 3, 0}},
		{57, []uint64{0, 4, 3, 2}},
		{58, []uint64{0, 4, 3, 2}},
		{59, []uint64{2, 4, 3, 0}},
		{60, []uint64{2, 4, 3, 0}},
		{61, []uint64{2, 4, 3, 0}},
		{62, []uint64{0, 4, 3, 2}},
		{63, []uint64{3, 4, 2, 0}},
		{64, []uint64{2, 4, 0, 3}},
		{65, []uint64{0, 3, 4, 2}},
		{66, []uint64{0, 3, 4, 2}},
		{67, []uint64{4, 2, 3, 0}},
		{68, []uint64{3, 0, 2, 4}},
		{69, []uint64{3, 0, 2, 4}},
		{70, []uint64{2, 4, 0, 3}},
		{71, []uint64{0, 4, 2, 3}},
		{72, []uint64{4, 3, 0, 2}},
		{73, []uint64{4, 3, 2, 0}},
		{74, []uint64{4, 3, 0, 2}},
		{75, []uint64{4, 3, 2, 0}},
		{76, []uint64{4, 2, 0, 3}},
		{77, []uint64{4, 0, 3, 2}},
		{78, []uint64{4, 0, 3, 2}},
		{79, []uint64{4, 3, 2, 0}},
		// epoch 2
		{80, []uint64{0, 4, 2, 3}},
		{81, []uint64{0, 4, 2, 3}},
		{82, []uint64{2, 4, 0, 3}},
		{83, []uint64{2, 4, 0, 3}},
		{84, []uint64{0, 4, 2, 3}},
		{85, []uint64{3, 4, 0, 2}},
		{86, []uint64{3, 4, 2, 0}},
		{87, []uint64{3, 4, 0, 2}},
		{88, []uint64{2, 4, 3, 0}},
		{89, []uint64{0, 3, 2, 4}},
		{90, []uint64{3, 2, 0, 4}},
		{91, []uint64{2, 0, 4, 3}},
		{92, []uint64{0, 2, 4, 3}},
		{93, []uint64{2, 0, 4, 3}},
		{94, []uint64{0, 4, 3, 2}},
		{95, []uint64{0, 4, 3, 2}},
		{96, []uint64{4, 3, 2, 0}},
		{97, []uint64{4, 3, 0, 2}},
		{98, []uint64{4, 3, 0, 2}},
		{99, []uint64{4, 3, 0, 2}},
		{100, []uint64{4, 2, 3, 0}},
		{101, []uint64{3, 0, 2, 4}},
		{102, []uint64{3, 0, 2, 4}},
		{103, []uint64{2, 3, 0, 4}},
		{104, []uint64{0, 3, 2, 4}},
		{105, []uint64{4, 2, 0, 3}},
		{106, []uint64{4, 0, 2, 3}},
		{107, []uint64{4, 0, 2, 3}},
		{108, []uint64{4, 2, 0, 3}},
		{109, []uint64{4, 2, 0, 3}},
		{110, []uint64{3, 0, 4, 2}},
		{111, []uint64{3, 0, 4, 2}},
		{112, []uint64{2, 4, 3, 0}},
		{113, []uint64{0, 3, 2, 4}},
		{114, []uint64{4, 2, 0, 3}},
		{115, []uint64{4, 2, 0, 3}},
		{116, []uint64{3, 0, 4, 2}},
		{117, []uint64{2, 3, 4, 0}},
		{118, []uint64{0, 3, 4, 2}},
		{119, []uint64{2, 3, 4, 0}},
	}

	for _, tc := range testCases {
		for i, want := range tc.want {
			validator := validators[i]
			backoff := backOffTime(validators, stakes, epochPeriod, tc.block, validator)
			if backoff != want {
				t.Errorf("backoff mismatch, block %v, validator %v, got %v, want %v", tc.block, names[validator], backoff, want)
			}
		}
	}
}

func TestGetValidatorSchedule(t *testing.T) {
	testCases := []struct {
		block uint64
		want  string
	}{
		// epoch 1
		{40, "validator-3"},
		{41, "validator-0"},
		{42, "validator-2"},
		{43, "validator-3"},
		{44, "validator-0"},
		{45, "validator-0"},
		{46, "validator-2"},
		{47, "validator-0"},
		{48, "validator-0"},
		{49, "validator-3"},
		{50, "validator-0"},
		{51, "validator-1"},
		{52, "validator-1"},
		{53, "validator-1"},
		{54, "validator-2"},
		{55, "validator-0"},
		{56, "validator-3"},
		{57, "validator-0"},
		{58, "validator-0"},
		{59, "validator-3"},
		{60, "validator-3"},
		{61, "validator-3"},
		{62, "validator-0"},
		{63, "validator-3"},
		{64, "validator-2"},
		{65, "validator-0"},
		{66, "validator-0"},
		{67, "validator-3"},
		{68, "validator-1"},
		{69, "validator-1"},
		{70, "validator-2"},
		{71, "validator-0"},
		{72, "validator-2"},
		{73, "validator-3"},
		{74, "validator-2"},
		{75, "validator-3"},
		{76, "validator-2"},
		{77, "validator-1"},
		{78, "validator-1"},
		{79, "validator-3"},
		// epoch 2
		{80, "validator-0"},
		{81, "validator-0"},
		{82, "validator-2"},
		{83, "validator-2"},
		{84, "validator-0"},
		{85, "validator-2"},
		{86, "validator-3"},
		{87, "validator-2"},
		{88, "validator-3"},
		{89, "validator-0"},
		{90, "validator-2"},
		{91, "validator-1"},
		{92, "validator-0"},
		{93, "validator-1"},
		{94, "validator-0"},
		{95, "validator-0"},
		{96, "validator-3"},
		{97, "validator-2"},
		{98, "validator-2"},
		{99, "validator-2"},
		{100, "validator-3"},
		{101, "validator-1"},
		{102, "validator-1"},
		{103, "validator-2"},
		{104, "validator-0"},
		{105, "validator-2"},
		{106, "validator-1"},
		{107, "validator-1"},
		{108, "validator-2"},
		{109, "validator-2"},
		{110, "validator-1"},
		{111, "validator-1"},
		{112, "validator-3"},
		{113, "validator-0"},
		{114, "validator-2"},
		{115, "validator-2"},
		{116, "validator-1"},
		{117, "validator-3"},
		{118, "validator-0"},
		{119, "validator-3"},
	}

	for _, tc := range testCases {
		schedule := getValidatorSchedule(validators, stakes, epochPeriod, tc.block)
		got := names[schedule[tc.block]]
		if got != tc.want {
			t.Errorf("validator mismatch, block %v, got %v, want %v", tc.block, got, tc.want)
		}
	}
}

func TestGetValidatorBlocks(t *testing.T) {
	testCases := []struct {
		name       string
		startBlock uint64
		endBlock   uint64
		want       map[string]int64
	}{
		{
			"epoch1",
			40,
			79,
			map[string]int64{
				"validator-0": 13,
				"validator-1": 7,
				"validator-2": 8,
				"validator-3": 12,
			},
		},
		{
			"epoch2",
			80,
			119,
			map[string]int64{
				"validator-0": 10,
				"validator-1": 9,
				"validator-2": 14,
				"validator-3": 7,
			}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for block := tc.startBlock; block <= tc.endBlock; block++ {
				schedule := getValidatorSchedule(validators, stakes, epochPeriod, block)
				validators, blocks := getValidatorBlocks(schedule)

				for i, gotValidator := range validators {
					wantName, ok := names[gotValidator]
					if !ok {
						t.Errorf("unknown validator, block %v, validator %v", block, gotValidator)
					}

					gotBlocks := blocks[i]
					wantBlocks := big.NewInt(tc.want[wantName])
					if gotBlocks.Cmp(wantBlocks) != 0 {
						t.Errorf("blocks mitmatch, block %v, got %v, want %v", block, gotBlocks, wantBlocks)
					}
				}
			}
		})
	}
}
