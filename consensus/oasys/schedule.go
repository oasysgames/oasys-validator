package oasys

import (
	"errors"
	"math/big"
	"math/rand"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	lru "github.com/hashicorp/golang-lru"
)

var (
	allHashes       *lru.Cache
	lastBlockHashes *lru.Cache
	emptyHash       common.Hash
)

func init() {
	allHashes, _ = lru.New(65535 * 4)
	lastBlockHashes, _ = lru.New(65535)
}

// type validatorAndValue struct {
// 	validator common.Address
// 	value     *big.Int
// }

// type validatorsAndValuesAscending []*validatorAndValue

// func (s validatorsAndValuesAscending) Len() int { return len(s) }
// func (s validatorsAndValuesAscending) Less(i, j int) bool {
// 	if s[i].value.Cmp(s[j].value) == 0 {
// 		return bytes.Compare(s[i].validator[:], s[j].validator[:]) < 0
// 	}
// 	return s[i].value.Cmp(s[j].value) < 0
// }
// func (s validatorsAndValuesAscending) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// func sortValidatorsAndValues(validators []common.Address, values []*big.Int) ([]common.Address, []*big.Int) {
// 	choices := make([]*validatorAndValue, len(validators))
// 	for i, validator := range validators {
// 		choices[i] = &validatorAndValue{validator, values[i]}
// 	}
// 	sort.Sort(validatorsAndValuesAscending(choices))

// 	rvalidators := make([]common.Address, len(choices))
// 	rvalues := make([]*big.Int, len(choices))
// 	for i, c := range choices {
// 		rvalidators[i] = c.validator
// 		rvalues[i] = new(big.Int).Set(c.value)
// 	}
// 	return rvalidators, rvalues
// }

type weightedChooser struct {
	random     *rand.Rand
	validators []common.Address
	totals     []int
	max        int
}

func (c *weightedChooser) choice() common.Address {
	if (c.max) == 0 {
		i := rand.Intn(len(c.validators))
		return c.validators[i]
	}

	x := c.randInt()
	i := 0
	j := len(c.totals)

	for i < j {
		h := (i + j) >> 1
		if c.totals[h] < x {
			i = h + 1
		} else {
			j = h
		}
	}

	return c.validators[i]
}

func (c *weightedChooser) randInt() int {
	if (c.max) == 0 {
		return 0
	}
	return c.random.Intn(c.max) + 1
}

func (c *weightedChooser) skip() {
	c.randInt()
}

func newWeightedChooser(validators []common.Address, stakes []*big.Int, seed int64) *weightedChooser {
	validators, stakes = sortValidatorsAndValues(validators, stakes)
	chooser := &weightedChooser{
		random:     rand.New(rand.NewSource(seed)),
		validators: make([]common.Address, len(validators)),
		totals:     make([]int, len(stakes)),
		max:        0,
	}
	for i, validator := range validators {
		chooser.validators[i] = validator
		chooser.max += int(new(big.Int).Div(stakes[i], ether).Int64())
		chooser.totals[i] = chooser.max
	}
	return chooser
}

func newWeightedChooserFromHeaderSeed(
	config *params.OasysConfig,
	chain consensus.ChainHeaderReader,
	validators []common.Address,
	stakes []*big.Int,
	env *environmentValue,
	header *types.Header,
) (*weightedChooser, error) {
	start := env.GetFirstBlock(header.Number.Uint64())
	seed := int64(start)
	if start > 0 {
		if seedHash, err := getPrevEpochLastBlockHash(config, chain, env, header); err != nil {
			return nil, err
		} else if seedHash != emptyHash {
			seed = seedHash.Big().Int64()
		}
	}
	return newWeightedChooser(validators, stakes, seed), nil
}

// func getValidatorSchedule(env *environmentValue, chooser *weightedChooser, number uint64) map[uint64]common.Address {
// 	epochPeriod := env.EpochPeriod.Uint64()
// 	start := env.GetFirstBlock(number)

// 	ret := make(map[uint64]common.Address)
// 	for i := uint64(0); i < epochPeriod; i++ {
// 		ret[start+i] = chooser.choice()
// 	}
// 	return ret
// }

// func backOffTime(env *environmentValue, chooser *weightedChooser, number uint64, validator common.Address) uint64 {
// 	for i := number - env.GetFirstBlock(number); i > 0; i-- {
// 		chooser.skip()
// 	}

// 	turn := 0
// 	prevs := make(map[common.Address]bool)
// 	for {
// 		picked := chooser.choice()
// 		if picked == validator {
// 			break
// 		}
// 		if prevs[picked] {
// 			continue
// 		}
// 		prevs[picked] = true
// 		turn++
// 	}

// 	if turn == 0 {
// 		return 0
// 	}
// 	return uint64(turn) + backoffWiggleTime
// }

func getPrevEpochLastBlockHash(
	config *params.OasysConfig,
	chain consensus.ChainHeaderReader,
	env *environmentValue,
	header *types.Header,
) (common.Hash, error) {
	if header == nil || header.Hash() == emptyHash {
		return emptyHash, errors.New("invalid header")
	}

	number := header.Number.Uint64()
	if number < config.Epoch {
		return emptyHash, nil // Previous epoch does not exists.
	}

	var (
		current   = header.Hash()
		parent    = header.ParentHash
		prevLast  = env.GetFirstBlock(number) - 1
		addCaches []string
	)
	for ; number > prevLast; number-- {
		key := cacheKey(current, number)

		// Cache all hashes that may be requested that are not yet stored in the *core.Blockchain.
		allHashes.ContainsOrAdd(key, parent)

		if cache, ok := lastBlockHashes.Get(key); ok {
			current = cache.(common.Hash)
			break
		}
		addCaches = append(addCaches, key)

		// Traverse the chain based on the ParentHash.
		if cache, ok := allHashes.Get(cacheKey(parent, number-1)); ok {
			current = parent
			parent = cache.(common.Hash)
			continue
		}
		if h := chain.GetHeader(parent, number-1); h != nil {
			current = h.Hash()
			parent = h.ParentHash
			continue
		}

		// Something is wrong.
		log.Error("Unable to traverse the chain",
			"parent", parent.Hex(), "number", number-1,
			"header.Number", header.Number, "header.Hash", header.Hash().Hex(), "header.ParentHash", header.ParentHash.Hex())
		return emptyHash, consensus.ErrUnknownAncestor
	}

	for _, key := range addCaches {
		lastBlockHashes.Add(key, current)
	}

	return current, nil
}

func cacheKey(hash common.Hash, number uint64) string {
	return strings.Join([]string{hash.Hex(), strconv.FormatUint(number, 10)}, ":")
}
