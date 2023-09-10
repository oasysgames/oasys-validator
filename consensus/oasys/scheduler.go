package oasys

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
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

type scheduler struct {
	env       *environmentValue
	number    uint64
	chooser   *weightedChooser
	exists    map[common.Address]bool
	schedules map[uint64]common.Address // block => validator
	turns     map[common.Address]uint64 // validator => turn
}

func newScheduler(env *environmentValue, validators []common.Address, number uint64, chooser *weightedChooser) *scheduler {
	s := &scheduler{
		env:       env,
		number:    number,
		chooser:   chooser,
		exists:    map[common.Address]bool{},
		schedules: map[uint64]common.Address{},
		turns:     map[common.Address]uint64{},
	}

	for _, validator := range s.chooser.validators {
		s.exists[validator] = true
	}

	var (
		start  = s.env.GetFirstBlock(s.number)
		period = s.env.EpochPeriod.Uint64()
	)
	for i := uint64(0); i < period; i++ {
		choiced := s.chooser.random()
		s.schedules[start+i] = choiced
		if start+i >= s.number {
			if _, ok := s.turns[choiced]; !ok {
				s.turns[choiced] = uint64(len(s.turns))
			}
		}
	}
	return s
}

func (s *scheduler) onSchedule(validator common.Address) bool {
	return s.schedules[s.number] == validator
}

func (s *scheduler) difficulty(validator common.Address, ext bool) *big.Int {
	if !ext {
		if s.onSchedule(validator) {
			return new(big.Int).Set(diffInTurn)
		}
		return new(big.Int).Set(diffNoTurn)
	}

	turn, err := s.turn(validator)
	if errors.Is(err, errUnauthorizedValidator) {
		return new(big.Int).Set(diffNoTurn)
	}

	unit := new(big.Int).Div(totalSupply, s.env.ValidatorThreshold).Uint64()
	vals := uint64(len(s.chooser.validators))
	return new(big.Int).SetUint64(unit * (vals - turn))
}

func (s *scheduler) turn(validator common.Address) (uint64, error) {
	if !s.exists[validator] {
		return 0, errUnauthorizedValidator
	}

	for {
		if _, ok := s.turns[validator]; ok {
			break
		}

		choiced := s.chooser.random()
		if _, ok := s.turns[choiced]; !ok {
			s.turns[choiced] = uint64(len(s.turns))
		}
	}

	return s.turns[validator], nil
}

func (s *scheduler) backOffTime(validator common.Address) uint64 {
	turn, err := s.turn(validator)
	if errors.Is(err, errUnauthorizedValidator) || turn == 0 {
		return 0
	}
	return turn + backoffWiggleTime
}

type validatorAndStake struct {
	validator common.Address
	stake     *big.Int
}

type validatorsAndValuesAscending []*validatorAndStake

func (s validatorsAndValuesAscending) Len() int { return len(s) }
func (s validatorsAndValuesAscending) Less(i, j int) bool {
	if s[i].stake.Cmp(s[j].stake) == 0 {
		return bytes.Compare(s[i].validator[:], s[j].validator[:]) < 0
	}
	return s[i].stake.Cmp(s[j].stake) < 0
}
func (s validatorsAndValuesAscending) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func sortValidatorsAndValues(validators []common.Address, stakes []*big.Int) ([]common.Address, []*big.Int) {
	choices := make([]*validatorAndStake, len(validators))
	for i, validator := range validators {
		choices[i] = &validatorAndStake{validator, stakes[i]}
	}
	sort.Sort(validatorsAndValuesAscending(choices))

	rvalidators := make([]common.Address, len(choices))
	rvalues := make([]*big.Int, len(choices))
	for i, c := range choices {
		rvalidators[i] = c.validator
		rvalues[i] = new(big.Int).Set(c.stake)
	}
	return rvalidators, rvalues
}

type weightedChooser struct {
	rnd        *rand.Rand
	validators []common.Address
	totals     []int
	max        int
}

func (c *weightedChooser) random() common.Address {
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
	return c.rnd.Intn(c.max) + 1
}

func newWeightedChooser(validators []common.Address, stakes []*big.Int, seed int64) *weightedChooser {
	validators, stakes = sortValidatorsAndValues(validators, stakes)
	chooser := &weightedChooser{
		rnd:        rand.New(rand.NewSource(seed)),
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
		return emptyHash, fmt.Errorf(
			"unable to traverse the chain: parent=%s, number=%d, header.Number=%d, header.Hash=%s, header.ParentHash=%s",
			parent, number-1, header.Number, header.Hash(), header.ParentHash)
	}

	for _, key := range addCaches {
		lastBlockHashes.Add(key, current)
	}

	return current, nil
}

func cacheKey(hash common.Hash, number uint64) string {
	return strings.Join([]string{hash.Hex(), strconv.FormatUint(number, 10)}, ":")
}
