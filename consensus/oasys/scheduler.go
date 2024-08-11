package oasys

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	lru "github.com/hashicorp/golang-lru"
)

var (
	emptyHash common.Hash

	// During batch validation, since the parent header does
	// not exist in the database, it is temporarily stored here.
	uncommittedHashes *lru.Cache

	// Cache the hash value of the final block of the
	// previous epoch for a block with a certain ParentHash.
	lastBlockHashes *lru.Cache
)

func init() {
	// Set the capacity equal to the "blockCacheMaxItems" in the "eth/downloader" package.
	// WARNING: The capacity must not be smaller than the maximum size of the batch verification.
	uncommittedHashes, _ = lru.New(8192)

	// Set the capacity equal to the maximum number of validators.
	// (That is equal to the maximum number of fork chains.)
	lastBlockHashes, _ = lru.New(1000)
}

type scheduler struct {
	mu  sync.Mutex
	env *environmentValue

	// WARNING: Consensus engine should not use this value directly.
	chooser *weightedChooser

	// Mapping the real address to the pointer address of "chooser.validators".
	ptrmap map[common.Address]*common.Address

	// Validator schedule indexed by block position in the epoch.
	// WARNING: Consensus engine should not use this value directly.
	choices []*common.Address

	// Validator order indexed by block position in the epoch.
	// WARNING: Consensus engine should not use this value directly.
	turns *lru.Cache

	// Example: period=1000, epoch=2, validators=[A,B,C]
	//   Index 0: block=1000 choices[0]=A turns[0]=[A,B,C]
	//   Index 1: block=1001 choices[1]=B turns[1]=[B,C,A]
	//   Index 2: block=1002 choices[2]=C turns[2]=[C,A,B]
	//   Index 3: block=1003 choices[3]=D turns[3]=[A,B,C]
	//   Note: The turn is random, so it will not always be [A,B,C].
}

func newScheduler(env *environmentValue, epochStart uint64, chooser *weightedChooser) *scheduler {
	period := env.EpochPeriod.Uint64()
	s := &scheduler{
		env:     env,
		chooser: chooser,
		ptrmap:  map[common.Address]*common.Address{},
		choices: make([]*common.Address, period),
	}
	s.turns, _ = lru.New(32)

	for i, addr := range s.chooser.validators {
		s.ptrmap[addr] = &s.chooser.validators[i]
	}

	for bpos := uint64(0); bpos < period; bpos++ {
		s.choices[bpos] = s.ptrmap[s.chooser.random()]
	}
	return s
}

func (s *scheduler) exists(validator common.Address) bool {
	return s.ptrmap[validator] != nil
}

func (s *scheduler) schedules() []*common.Address {
	return s.choices[:s.env.EpochPeriod.Uint64()]
}

func (s *scheduler) expect(number uint64) *common.Address {
	return s.schedules()[number-s.env.EpochStartBlock(number)]
}

func (s *scheduler) difficulty(number uint64, validator common.Address, ext bool) *big.Int {
	if !ext {
		if *s.expect(number) == validator {
			return new(big.Int).Set(diffInTurn)
		}
		return new(big.Int).Set(diffNoTurn)
	}

	turn, err := s.turn(number, validator)
	if err != nil {
		return new(big.Int).Set(diffNoTurn)
	}

	// Use the maximum number of validators as the minimum value of difficulty.
	// Also, the higher the priority of the validator in the block, the larger the difficulty.
	// In other words, during a reorganization, fork chains that closely resemble
	// the calculated validator schedule are more likely to be adopted as the main chain.
	minDiff := new(big.Int).Div(totalSupply, s.env.ValidatorThreshold)
	priority := new(big.Int).SetUint64(uint64(len(s.chooser.validators)) - turn)
	return new(big.Int).Mul(minDiff, priority)
}

func (s *scheduler) turn(number uint64, validator common.Address) (uint64, error) {
	if !s.exists(validator) {
		return 0, errUnauthorizedValidator
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	turns := map[*common.Address]uint64{}
	if cache, ok := s.turns.Get(number); ok {
		turns = cache.(map[*common.Address]uint64)
	} else {
		s.turns.Add(number, turns)
	}

	for bpos := number - s.env.EpochStartBlock(number); ; bpos++ {
		if turn, ok := turns[s.ptrmap[validator]]; ok {
			return turn, nil
		}
		if bpos >= uint64(len(s.choices)) {
			// Out of the first calculated schedule.
			s.choices = append(s.choices, s.ptrmap[s.chooser.random()])
		}
		if _, ok := turns[s.choices[bpos]]; !ok {
			turns[s.choices[bpos]] = uint64(len(turns))
		}
	}
}

func (s *scheduler) backOffTime(number uint64, validator common.Address) uint64 {
	turn, err := s.turn(number, validator)
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

func sortValidatorsAndValues(
	validators []common.Address,
	stakes []*big.Int,
) ([]common.Address, []*big.Int) {
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
	mu         sync.Mutex
	rnd        *rand.Rand
	validators []common.Address
	totals     []int
	max        int
}

// Return a validator to the scheduler at weighted random based on stake amount.
// First, at the time a new Scheduler is created, it is called
// for the number of blocks in an epoch to determine the validator schedule.
// Next, when the "scheduler.turn()" method is called to determine the validation
// order in a particular block of a certain validator, if it is outside the first
// calculated schedule, it will be called repeatedly until it is found.
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if (c.max) == 0 {
		return 0
	}
	return c.rnd.Intn(c.max) + 1
}

func newWeightedChooser(
	validators []common.Address,
	stakes []*big.Int,
	seed int64,
) *weightedChooser {
	validators, stakes = sortValidatorsAndValues(validators, stakes)
	chooser := &weightedChooser{
		rnd:        rand.New(rand.NewSource(seed)),
		validators: make([]common.Address, len(validators)),
		totals:     make([]int, len(stakes)),
		max:        0,
	}
	// The scheduler uses pointers, so copy it just in case.
	copy(chooser.validators, validators)

	for i := range stakes {
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
	var (
		number     = header.Number.Uint64()
		epochStart = env.EpochStartBlock(number)
		parent     = header.ParentHash
	)
	for ; number > epochStart; number-- {
		if cache, ok := lastBlockHashes.Get(parent); ok {
			parent = cache.(common.Hash)
			break
		}

		if chain.GetCanonicalHash(number-1) == parent {
			// Reached the header of the canonical chain.
			if h := chain.GetHeaderByNumber(epochStart); h != nil {
				parent = h.ParentHash
				break
			}
			return emptyHash, consensus.ErrUnknownAncestor
		}

		if h := chain.GetHeader(parent, number-1); h != nil {
			// Reached the committed header (Not necessarily the canonical chain).
			parent = h.ParentHash
		} else if uncommitted, ok := uncommittedHashes.Get(parent); ok {
			// Not committed to the database.
			parent = uncommitted.(common.Hash)
		} else {
			// Something is wrong.
			return emptyHash, fmt.Errorf(
				"unable to traverse the chain: parent=%s, number=%d, header.Number=%d, header.Hash=%s, header.ParentHash=%s",
				parent, number-1, header.Number, header.Hash(), header.ParentHash)
		}
	}

	lastBlockHashes.ContainsOrAdd(header.ParentHash, parent)
	return parent, nil
}
