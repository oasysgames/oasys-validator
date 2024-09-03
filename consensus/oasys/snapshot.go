package oasys

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
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
	ethAPI   *ethapi.BlockChainAPI

	Number      uint64                            `json:"number"`                // Block number where the snapshot was created
	Hash        common.Hash                       `json:"hash"`                  // Block hash where the snapshot was created
	Validators  map[common.Address]*ValidatorInfo `json:"validators"`            // Set of authorized validators and stakes at this moment
	Attestation *types.VoteData                   `json:"attestation:omitempty"` // Attestation for fast finality, but `Source` used as `Finalized`

	Environment *params.EnvironmentValue `json:"environment"`
}

type ValidatorInfo struct {
	// The index is determined by the sorted order of the validator owner address
	Stake       *big.Int           `json:"stake:omitempty"`        // The stake amount
	Index       int                `json:"index:omitempty"`        // The index should offset by 1
	VoteAddress types.BLSPublicKey `json:"vote_address,omitempty"` // The vote address
}

// validatorsAscending implements the sort interface to allow sorting a list of addresses
type validatorsAscending []common.Address

func (s validatorsAscending) Len() int           { return len(s) }
func (s validatorsAscending) Less(i, j int) bool { return bytes.Compare(s[i][:], s[j][:]) < 0 }
func (s validatorsAscending) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// newSnapshot creates a new snapshot with the specified startup parameters. This
// method does not initialize the set of recent validators, so only ever use if for
// the genesis block.
func newSnapshot(config *params.ChainConfig, sigcache *lru.ARCCache, ethAPI *ethapi.BlockChainAPI,
	number uint64, hash common.Hash, validators []common.Address, environment *params.EnvironmentValue) *Snapshot {
	snap := &Snapshot{
		config:      config,
		sigcache:    sigcache,
		ethAPI:      ethAPI,
		Number:      number,
		Hash:        hash,
		Validators:  make(map[common.Address]*ValidatorInfo),
		Environment: environment.Copy(),
	}
	for i, address := range validators {
		snap.Validators[address] = &ValidatorInfo{
			Index: i + 1,
			Stake: new(big.Int).Set(common.Big0),
		}
	}
	return snap
}

// loadSnapshot loads an existing snapshot from the database.
func loadSnapshot(config *params.ChainConfig, sigcache *lru.ARCCache, ethAPI *ethapi.BlockChainAPI,
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
		Validators:  make(map[common.Address]*ValidatorInfo),
		Environment: s.Environment.Copy(),
	}
	for address, info := range s.Validators {
		var voteAddress types.BLSPublicKey
		copy(voteAddress[:], info.VoteAddress.Bytes())
		cpy.Validators[address] = &ValidatorInfo{
			Index:       info.Index,
			Stake:       new(big.Int).Set(info.Stake),
			VoteAddress: voteAddress,
		}
	}
	if s.Attestation != nil {
		cpy.Attestation = &types.VoteData{
			SourceNumber: s.Attestation.SourceNumber,
			SourceHash:   s.Attestation.SourceHash,
			TargetNumber: s.Attestation.TargetNumber,
			TargetHash:   s.Attestation.TargetHash,
		}
	}
	return cpy
}

func (s *Snapshot) updateAttestation(header *types.Header, chainConfig *params.ChainConfig, oasysConfig *params.OasysConfig) {
	if !chainConfig.IsFastFinalityEnabled(header.Number) {
		return
	}

	// The attestation should have been checked in verify header, update directly
	attestation, _ := getVoteAttestationFromHeader(header, chainConfig, oasysConfig, s.Environment)
	if attestation == nil {
		return
	}

	// Headers with bad attestation are accepted before Plato upgrade,
	// but Attestation of snapshot is only updated when the target block is direct parent of the header
	targetNumber := attestation.Data.TargetNumber
	targetHash := attestation.Data.TargetHash
	if targetHash != header.ParentHash || targetNumber+1 != header.Number.Uint64() {
		log.Warn("updateAttestation failed", "error", fmt.Errorf("invalid attestation, target mismatch, expected block: %d, hash: %s; real block: %d, hash: %s",
			header.Number.Uint64()-1, header.ParentHash, targetNumber, targetHash))
		updateAttestationErrorCounter.Inc(1)
		return
	}

	// Update attestation
	// Two scenarios for s.Attestation being nil:
	// 1) The first attestation is assembled.
	// 2) The snapshot on disk is missing, prompting the creation of a new snapshot using `newSnapshot`.
	if s.Attestation != nil && attestation.Data.SourceNumber+1 != attestation.Data.TargetNumber {
		s.Attestation.TargetNumber = attestation.Data.TargetNumber
		s.Attestation.TargetHash = attestation.Data.TargetHash
	} else {
		s.Attestation = attestation.Data
	}
}

// apply creates a new authorization snapshot by applying the given headers to
// the original one.
func (s *Snapshot) apply(headers []*types.Header, chain consensus.ChainHeaderReader, oasysConfig *params.OasysConfig) (*Snapshot, error) {
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
		if number > 0 && snap.Environment.IsEpoch(number) {
			var nextValidator *nextValidators
			if s.config.IsFastFinalityEnabled(header.Number) {
				if nextValidator, err = getValidatorsFromHeader(header); err != nil {
					log.Warn("failed to get validators from header", "in", "Snapshot.apply", "hash", header.Hash(), "number", number, "err", err)
				}
			}
			if nextValidator == nil {
				if nextValidator, err = getNextValidators(s.config, s.ethAPI, header.ParentHash, snap.Environment.Epoch(number), number); err != nil {
					return nil, fmt.Errorf("failed to get validators, in Snapshot.apply, err: %w", err)
				}
				nextValidator.SortByOwner() // sort by owner for fast finality
			}
			var nextEnv *params.EnvironmentValue
			if s.config.IsFastFinalityEnabled(header.Number) {
				nextEnv, err = getEnvironmentFromHeader(header)
			} else {
				nextEnv, err = getNextEnvironmentValue(s.ethAPI, header.ParentHash)
			}
			if err != nil {
				log.Error("Failed to get environment value", "in", "Snapshot.apply", "hash", header.ParentHash, "number", number, "err", err)
				return nil, err
			}

			snap.Environment = nextEnv.Copy()
			snap.Validators = make(map[common.Address]*ValidatorInfo, len(nextValidator.Operators))
			for i, address := range nextValidator.Operators {
				voterIndex := i + 1
				snap.Validators[address] = &ValidatorInfo{
					Index:       voterIndex,
					Stake:       nextValidator.Stakes[i],
					VoteAddress: nextValidator.VoteAddresses[i],
				}
			}

			exists = nextValidator.Exists(validator)
		} else {
			exists = snap.exists(validator)
		}

		if !exists {
			return nil, errUnauthorizedValidator
		}

		snap.updateAttestation(header, s.config, oasysConfig)
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

func (s *Snapshot) ToNextValidators() *nextValidators {
	operators := make([]common.Address, len(s.Validators))
	stakes := make([]*big.Int, len(s.Validators))
	voteAddresses := make([]types.BLSPublicKey, len(s.Validators))
	for address, info := range s.Validators {
		// No worry, voterIndex is assured when creating snapshot
		i := info.Index - 1
		operators[i] = address
		stakes[i] = new(big.Int).Set(info.Stake)
		copy(voteAddresses[i][:], info.VoteAddress[:])
	}
	return &nextValidators{
		Owners:        make([]common.Address, len(operators)), // take care the owners are empty
		Operators:     operators,
		Stakes:        stakes,
		VoteAddresses: voteAddresses,
	}
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
