package oasys

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	lru "github.com/hashicorp/golang-lru"
)

// Snapshot is the state of the authorization voting at a given point in time.
type Snapshot struct {
	config   *params.ChainConfig // Consensus engine parameters to fine tune behavior
	sigcache *lru.ARCCache       // Cache of recent block signatures to speed up ecrecover
	ethAPI   *ethapi.PublicBlockChainAPI

	Number     uint64                      `json:"number"`     // Block number where the snapshot was created
	Hash       common.Hash                 `json:"hash"`       // Block hash where the snapshot was created
	Validators map[common.Address]*big.Int `json:"validators"` // Set of authorized validators and stakes at this moment

	Environment *environmentValue `json:"environment"`
}

// validatorsAscending implements the sort interface to allow sorting a list of addresses
type validatorsAscending []common.Address

func (s validatorsAscending) Len() int           { return len(s) }
func (s validatorsAscending) Less(i, j int) bool { return bytes.Compare(s[i][:], s[j][:]) < 0 }
func (s validatorsAscending) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// newSnapshot creates a new snapshot with the specified startup parameters. This
// method does not initialize the set of recent validators, so only ever use if for
// the genesis block.
func newSnapshot(config *params.ChainConfig, sigcache *lru.ARCCache, ethAPI *ethapi.PublicBlockChainAPI,
	number uint64, hash common.Hash, validators []common.Address, environment *environmentValue) *Snapshot {
	snap := &Snapshot{
		config:      config,
		sigcache:    sigcache,
		ethAPI:      ethAPI,
		Number:      number,
		Hash:        hash,
		Validators:  make(map[common.Address]*big.Int),
		Environment: environment.Copy(),
	}
	for _, address := range validators {
		snap.Validators[address] = new(big.Int).Set(common.Big0)
	}
	return snap
}

// loadSnapshot loads an existing snapshot from the database.
func loadSnapshot(config *params.ChainConfig, sigcache *lru.ARCCache, ethAPI *ethapi.PublicBlockChainAPI,
	db ethdb.Database, hash common.Hash) (*Snapshot, error) {
	blob, err := db.Get(append([]byte("oasys-"), hash[:]...))
	if err != nil {
		return nil, err
	}
	snap := new(Snapshot)
	if err := json.Unmarshal(blob, snap); err != nil {
		return nil, err
	}
	snap.config = config
	snap.sigcache = sigcache
	snap.ethAPI = ethAPI

	return snap, nil
}

// store inserts the snapshot into the database.
func (s *Snapshot) store(db ethdb.Database) error {
	blob, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return db.Put(append([]byte("oasys-"), s.Hash[:]...), blob)
}

// copy creates a deep copy of the snapshot, though not the individual votes.
func (s *Snapshot) copy() *Snapshot {
	cpy := &Snapshot{
		config:      s.config,
		sigcache:    s.sigcache,
		ethAPI:      s.ethAPI,
		Number:      s.Number,
		Hash:        s.Hash,
		Validators:  make(map[common.Address]*big.Int),
		Environment: s.Environment.Copy(),
	}
	for address, stake := range s.Validators {
		cpy.Validators[address] = new(big.Int).Set(stake)
	}
	return cpy
}

// apply creates a new authorization snapshot by applying the given headers to
// the original one.
func (s *Snapshot) apply(headers []*types.Header, chain consensus.ChainHeaderReader, evm *vm.EVM) (*Snapshot, error) {
	// Allow passing in no headers for cleaner code
	if len(headers) == 0 {
		return s, nil
	}
	// Sanity check that the headers can be applied
	for i := 0; i < len(headers)-1; i++ {
		if headers[i+1].Number.Uint64() != headers[i].Number.Uint64()+1 {
			return nil, errInvalidChain
		}
	}
	if headers[0].Number.Uint64() != s.Number+1 {
		return nil, errInvalidChain
	}
	// Iterate through the headers and create a new snapshot
	snap := s.copy()

	for _, header := range headers {
		number := header.Number.Uint64()

		validator, err := ecrecover(header, s.sigcache)
		if err != nil {
			return nil, err
		}

		var exists bool
		if number > 0 && number%snap.Environment.EpochPeriod.Uint64() == 0 {
			nextValidator, err := getNextValidators(s.config, s.ethAPI, header.ParentHash, snap.Environment.Epoch(number), number, evm)
			if err != nil {
				log.Error("Failed to get validators", "in", "Snapshot.apply", "hash", header.ParentHash, "number", number, "err", err)
				return nil, err
			}
			nextEnv, err := getNextEnvironmentValue(s.ethAPI, header.ParentHash)
			if err != nil {
				log.Error("Failed to get environment value", "in", "Snapshot.apply", "hash", header.ParentHash, "number", number, "err", err)
				return nil, err
			}

			snap.Environment = nextEnv.Copy()
			snap.Validators = map[common.Address]*big.Int{}
			for i, address := range nextValidator.Operators {
				snap.Validators[address] = nextValidator.Stakes[i]
			}

			exists = nextValidator.Exists(validator)
		} else {
			exists = snap.exists(validator)
		}

		if !exists {
			return nil, errUnauthorizedValidator
		}
	}
	snap.Number += uint64(len(headers))
	snap.Hash = headers[len(headers)-1].Hash()

	return snap, nil
}

// validators retrieves the list of authorized validators in ascending order.
func (s *Snapshot) validators() []common.Address {
	validators := make([]common.Address, 0, len(s.Validators))
	for v := range s.Validators {
		validators = append(validators, v)
	}
	sort.Sort(validatorsAscending(validators))
	return validators
}

func (s *Snapshot) exists(validator common.Address) bool {
	_, ok := s.Validators[validator]
	return ok
}

func (s *Snapshot) getValidatorSchedule(chain consensus.ChainHeaderReader, env *environmentValue, number uint64) map[uint64]common.Address {
	validators, stakes := s.validatorsToTuple()
	return getValidatorSchedule(chain, validators, stakes, env, number)
}

func (s *Snapshot) getValidatorScheduleByHash(chain consensus.ChainHeaderReader, env *environmentValue, number uint64, hash common.Hash) map[uint64]common.Address {
	validators, stakes := s.validatorsToTuple()
	return getValidatorScheduleByHash(chain, validators, stakes, env, number, hash)
}

func (s *Snapshot) backOffTime(chain consensus.ChainHeaderReader, env *environmentValue, number uint64, validator common.Address) uint64 {
	if !s.exists(validator) {
		return 0
	}
	validators, stakes := s.validatorsToTuple()
	return backOffTime(chain, validators, stakes, env, number, validator)
}

func (s *Snapshot) validatorsToTuple() ([]common.Address, []*big.Int) {
	operators := make([]common.Address, len(s.Validators))
	stakes := make([]*big.Int, len(s.Validators))
	i := 0
	for address, stake := range s.Validators {
		operators[i] = address
		stakes[i] = new(big.Int).Set(stake)
		i++
	}
	return operators, stakes
}

func parseValidatorBytes(validatorBytes []byte) ([]common.Address, error) {
	if len(validatorBytes)%common.AddressLength != 0 {
		return nil, errors.New("invalid validator bytes")
	}
	n := len(validatorBytes) / common.AddressLength
	result := make([]common.Address, n)
	for i := 0; i < n; i++ {
		address := make([]byte, common.AddressLength)
		copy(address, validatorBytes[i*common.AddressLength:(i+1)*common.AddressLength])
		result[i] = common.BytesToAddress(address)
	}
	return result, nil
}
