package oasys

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/crypto/sha3"
)

const (
	checkpointInterval = 1024 // Number of blocks after which to save the vote snapshot to the database
	inmemorySnapshots  = 128  // Number of recent vote snapshots to keep in memory
	inmemorySignatures = 4096 // Number of recent block signatures to keep in memory

	backoffWiggleTime = uint64(1) // second
)

// Oasys proof-of-stake protocol constants.
var (
	epochLength = uint64(30000) // Default number of blocks after which to checkpoint and reset the pending votes

	extraVanity = 32                     // Fixed number of extra-data prefix bytes reserved for signer vanity
	extraSeal   = crypto.SignatureLength // Fixed number of extra-data suffix bytes reserved for signer seal

	uncleHash = types.CalcUncleHash(nil) // Always Keccak256(RLP([])) as uncles are meaningless outside of PoW.

	diffInTurn = big.NewInt(2) // Block difficulty for in-turn signatures
	diffNoTurn = big.NewInt(1) // Block difficulty for out-of-turn signatures

	ether       = big.NewInt(1_000_000_000_000_000_000)
	totalSupply = new(big.Int).Mul(big.NewInt(10_000_000_000), ether) // From WhitePaper

	BigMaxInt64 = big.NewInt(math.MaxInt64)
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	// errUnknownBlock is returned when the list of signers is requested for a block
	// that is not part of the local blockchain.
	errUnknownBlock = errors.New("unknown block")

	// errMissingVanity is returned if a block's extra-data section is shorter than
	// 32 bytes, which is required to store the signer vanity.
	errMissingVanity = errors.New("extra-data 32 byte vanity prefix missing")

	// errMissingSignature is returned if a block's extra-data section doesn't seem
	// to contain a 65 byte secp256k1 signature.
	errMissingSignature = errors.New("extra-data 65 byte signature suffix missing")

	// errExtraSigners is returned if non-checkpoint block contain signer data in
	// their extra-data fields.
	errExtraSigners = errors.New("non-checkpoint block contains extra signer list")

	// errInvalidCheckpointValidators is returned if a checkpoint block contains an
	// invalid list of validators (i.e. non divisible by 20 bytes).
	errInvalidCheckpointValidators = errors.New("invalid validator list on checkpoint block")

	// errInvalidEpochHash is returned if a epoch block contains an invalid Keccak256 hash.
	errInvalidEpochHash = errors.New("invalid hash on epoch block")

	// errMismatchingEpochValidators is returned if a checkpoint block contains a
	// list of validators different than the one the local node calculated.
	errMismatchingEpochValidators = errors.New("mismatching validator list on checkpoint block")

	// errMismatchingEpochHash is returned if a epoch block contains a
	// Keccak256 hash different than the one the local node calculated.
	errMismatchingEpochHash = errors.New("mismatching hash of validator list on epoch block")

	// errInvalidMixDigest is returned if a block's mix digest is non-zero.
	errInvalidMixDigest = errors.New("non-zero mix digest")

	// errInvalidUncleHash is returned if a block contains an non-empty uncle list.
	errInvalidUncleHash = errors.New("non empty uncle hash")

	// errInvalidDifficulty is returned if the difficulty of a block neither 1 or 2.
	errInvalidDifficulty = errors.New("invalid difficulty")

	// errWrongDifficulty is returned if the difficulty of a block doesn't match the
	// turn of the signer.
	errWrongDifficulty = errors.New("wrong difficulty")

	// errInvalidChain is returned if an authorization list is attempted to
	// be modified via out-of-range or non-contiguous headers.
	errInvalidChain = errors.New("out-of-range or non-contiguous headers")

	// errUnauthorizedValidator is returned if a header is signed by a non-authorized entity.
	errUnauthorizedValidator = errors.New("unauthorized validator")

	// errCoinBaseMisMatch is returned if a header's coinbase do not match with signature
	errCoinBaseMisMatch = errors.New("coinbase do not match with signature")
)

var (
	// The key is the hash value of the final block from the previous epoch.
	schedulerCache *lru.Cache
)

// SignerFn hashes and signs the data to be signed by a backing account.
type SignerFn func(signer accounts.Account, mimeType string, message []byte) ([]byte, error)
type TxSignerFn func(accounts.Account, *types.Transaction, *big.Int) (*types.Transaction, error)

func init() {
	// The capacity should be greater than or equal to the
	// maximum batch verification size divided by the epoch period.
	schedulerCache, _ = lru.New(32)
}

// ecrecover extracts the Ethereum account address from a signed header.
func ecrecover(header *types.Header, sigcache *lru.ARCCache) (common.Address, error) {
	// If the signature's already cached, return that
	hash := header.Hash()
	if address, known := sigcache.Get(hash); known {
		return address.(common.Address), nil
	}
	// Retrieve the signature from the header extra-data
	if len(header.Extra) < extraSeal {
		return common.Address{}, errMissingSignature
	}
	signature := header.Extra[len(header.Extra)-extraSeal:]

	// Recover the public key and the Ethereum address
	pubkey, err := crypto.Ecrecover(SealHash(header).Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])

	sigcache.Add(hash, signer)
	return signer, nil
}

// Oasys is the proof-of-stake consensus engine
type Oasys struct {
	chainConfig *params.ChainConfig // Chain config
	config      *params.OasysConfig // Consensus engine configuration parameters
	db          ethdb.Database      // Database to store and retrieve snapshot checkpoints

	recents    *lru.ARCCache // Snapshots for recent block to speed up reorgs
	signatures *lru.ARCCache // Signatures of recent blocks to speed up mining

	proposals map[common.Address]bool // Current list of proposals we are pushing

	signer common.Address // Ethereum address of the signing key
	signFn SignerFn       // Signer function to authorize hashes with
	lock   sync.RWMutex   // Protects the signer fields

	ethAPI   *ethapi.BlockChainAPI
	txSigner types.Signer
	txSignFn TxSignerFn

	// The fields below are for testing only
	fakeDiff bool // Skip difficulty verifications
}

// New creates a Oasys proof-of-stake consensus engine with the initial
// signers set to the ones provided by the user.
func New(chainConfig *params.ChainConfig, config *params.OasysConfig, db ethdb.Database, ethAPI *ethapi.BlockChainAPI) *Oasys {
	// Set any missing consensus parameters to their defaults
	conf := *config
	if conf.Epoch == 0 {
		conf.Epoch = epochLength
	}
	// Allocate the snapshot caches and create the engine
	recents, _ := lru.NewARC(inmemorySnapshots)
	signatures, _ := lru.NewARC(inmemorySignatures)

	return &Oasys{
		chainConfig: chainConfig,
		config:      &conf,
		db:          db,
		recents:     recents,
		signatures:  signatures,
		proposals:   make(map[common.Address]bool),
		ethAPI:      ethAPI,
		txSigner:    types.MakeSigner(chainConfig, common.Big0, 0),
	}
}

// Author implements consensus.Engine, returning the Ethereum address recovered
// from the signature in the header's extra-data section.
func (c *Oasys) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (c *Oasys) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header) error {
	return c.verifyHeader(chain, header, nil)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers. The
// method returns a quit channel to abort the operations and a results channel to
// retrieve the async verifications (the order is that of the input slice).
func (c *Oasys) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			uncommittedHashes.Add(header.Hash(), header.ParentHash)
			err := c.verifyHeader(chain, header, headers[:i])

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

// verifyHeader checks whether a header conforms to the consensus rules.The
// caller may optionally pass in a batch of parents (ascending order) to avoid
// looking those up from the database. This is useful for concurrently verifying
// a batch of new headers.
func (c *Oasys) verifyHeader(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) error {
	if header.Number == nil {
		return errUnknownBlock
	}
	number := header.Number.Uint64()

	// Don't waste time checking blocks from the future
	if header.Time > uint64(time.Now().Unix()) {
		return consensus.ErrFutureBlock
	}
	// Check that the extra-data contains both the validators and signature
	if len(header.Extra) < extraVanity {
		return errMissingVanity
	}
	if len(header.Extra) < extraVanity+extraSeal {
		return errMissingSignature
	}
	// Ensure that the extra-data contains a validator list on checkpoint, but none otherwise
	env, _ := getEnvironmentValue(c.chainConfig, number)
	validatorBytes := len(header.Extra) - extraVanity - extraSeal
	if env.IsEpochStartBlock(number) {
		if err := c.verifyExtraHeaderLengthInEpoch(header.Number, validatorBytes); err != nil {
			return err
		}
	} else if validatorBytes != 0 {
		return errExtraSigners
	}
	// Ensure that the mix digest is zero as we don't have fork protection currently
	if header.MixDigest != (common.Hash{}) {
		return errInvalidMixDigest
	}
	// Ensure that the block doesn't contain any uncles which are meaningless in PoS
	if header.UncleHash != uncleHash {
		return errInvalidUncleHash
	}
	// Ensure that the block's difficulty is meaningful (may not be correct at this point)
	if number > 0 && header.Difficulty == nil {
		return errInvalidDifficulty
	}
	// Verify that the gas limit is <= 2^63-1
	if header.GasLimit > params.MaxGasLimit {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, params.MaxGasLimit)
	}
	// Verify the non-existence of withdrawalsHash.
	if header.WithdrawalsHash != nil {
		return fmt.Errorf("invalid withdrawalsHash: have %x, expected nil", header.WithdrawalsHash)
	}
	if chain.Config().IsCancun(header.Number, header.Time) {
		return errors.New("oasys does not support cancun fork")
	}
	// Verify the non-existence of cancun-specific header fields
	switch {
	case header.ExcessBlobGas != nil:
		return fmt.Errorf("invalid excessBlobGas: have %d, expected nil", header.ExcessBlobGas)
	case header.BlobGasUsed != nil:
		return fmt.Errorf("invalid blobGasUsed: have %d, expected nil", header.BlobGasUsed)
	case header.ParentBeaconRoot != nil:
		return fmt.Errorf("invalid parentBeaconRoot, have %#x, expected nil", header.ParentBeaconRoot)
	}
	// All basic checks passed, verify cascading fields
	return c.verifyCascadingFields(chain, header, parents, env)
}

// verifyCascadingFields verifies all the header fields that are not standalone,
// rather depend on a batch of previous headers. The caller may optionally pass
// in a batch of parents (ascending order) to avoid looking those up from the
// database. This is useful for concurrently verifying a batch of new headers.
func (c *Oasys) verifyCascadingFields(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header, env *environmentValue) error {
	// The genesis block is the always valid dead-end
	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}

	// Ensure that the block's timestamp isn't too close to its parent
	var parent *types.Header
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, number-1)
	}
	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}

	// Ensure that the block's timestamp is older than the scheduled validator backoff time
	var (
		validators []common.Address
		stakes     []*big.Int
	)
	if number > 0 && env.IsEpochStartBlock(number) {
		// TODO: Extract the validators from header extra data
		// Now the keccak256 hash of the validators is stored in the extra data.
		// To avoid ethapi call, we need to store each validator's address and stake amount.
		result, err := getNextValidators(c.chainConfig, c.ethAPI, header.ParentHash, env.Epoch(number), number)
		if err != nil {
			log.Error("Failed to get validators", "in", "verifyCascadingFields", "hash", header.ParentHash, "number", number, "err", err)
			return err
		}
		validators, stakes = result.Operators, result.Stakes
	} else {
		// Retrieve the snapshot needed to verify this header and cache it
		snap, err := c.snapshot(chain, number-1, header.ParentHash, parents)
		if err != nil {
			return err
		}
		validators, stakes = snap.validatorsToTuple()
	}
	scheduler, err := c.scheduler(chain, header, env, validators, stakes)
	if err != nil {
		log.Error("Failed to get scheduler", "in", "verifyCascadingFields", "number", number, "err", err)
		return err
	}
	if header.Time < parent.Time+env.BlockPeriod.Uint64()+scheduler.backOffTime(number, header.Coinbase) {
		return consensus.ErrFutureBlock
	}

	// Verify that the gasUsed is <= gasLimit
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}
	if !chain.Config().IsLondon(header.Number) {
		// Verify BaseFee not present before EIP-1559 fork.
		if header.BaseFee != nil {
			return fmt.Errorf("invalid baseFee before fork: have %d, want <nil>", header.BaseFee)
		}
		if err := misc.VerifyGaslimit(parent.GasLimit, header.GasLimit); err != nil {
			return err
		}
	} else if err := eip1559.VerifyEIP1559Header(chain.Config(), parent, header); err != nil {
		// Verify the header's EIP-1559 attributes.
		return err
	}

	// All basic checks passed, verify the seal and return
	return c.verifySeal(chain, header, parents, scheduler)
}

// snapshot retrieves the authorization snapshot at a given point in time.
func (c *Oasys) snapshot(chain consensus.ChainHeaderReader, number uint64, hash common.Hash, parents []*types.Header) (*Snapshot, error) {
	// Search for a snapshot in memory or on disk for checkpoints
	var (
		headers []*types.Header
		snap    *Snapshot
	)
	for snap == nil {
		// If an in-memory snapshot was found, use that
		if s, ok := c.recents.Get(hash); ok {
			snap = s.(*Snapshot)
			break
		}
		// If an on-disk checkpoint snapshot can be found, use that
		if number%checkpointInterval == 0 {
			if s, err := loadSnapshot(c.chainConfig, c.signatures, c.ethAPI, c.db, hash); err == nil {
				log.Trace("Loaded snapshot from disk", "number", number, "hash", hash)
				snap = s
				break
			}
		}
		// If we're at the genesis, snapshot the initial state. Alternatively if we're
		// at a checkpoint block without a parent (light client CHT), or we have piled
		// up more headers than allowed to be reorged (chain reinit from a freezer),
		// consider the checkpoint trusted and snapshot it.
		if number == 0 {
			checkpoint := chain.GetHeaderByNumber(number)
			if checkpoint != nil {
				hash := checkpoint.Hash()

				validatorBytes := checkpoint.Extra[extraVanity : len(checkpoint.Extra)-extraSeal]
				validators, err := parseValidatorBytes(validatorBytes)
				if err != nil {
					return nil, err
				}

				snap = newSnapshot(c.chainConfig, c.signatures, c.ethAPI, number, hash, validators)
				if err := snap.store(c.db); err != nil {
					return nil, err
				}
				log.Info("Stored checkpoint snapshot to disk", "number", number, "hash", hash)
				break
			}
		}
		// No snapshot for this header, gather the header and move backward
		var header *types.Header
		if len(parents) > 0 {
			// If we have explicit parents, pick from there (enforced)
			header = parents[len(parents)-1]
			if header.Hash() != hash || header.Number.Uint64() != number {
				return nil, consensus.ErrUnknownAncestor
			}
			parents = parents[:len(parents)-1]
		} else {
			// No explicit parents (or no more left), reach out to the database
			header = chain.GetHeader(hash, number)
			if header == nil {
				return nil, consensus.ErrUnknownAncestor
			}
		}
		headers = append(headers, header)
		number, hash = number-1, header.ParentHash
	}
	if snap == nil {
		return nil, fmt.Errorf("unknown error while retrieving snapshot at block %v", number)
	}

	// Previous snapshot found, apply any pending headers on top of it
	for i := 0; i < len(headers)/2; i++ {
		headers[i], headers[len(headers)-1-i] = headers[len(headers)-1-i], headers[i]
	}
	snap, err := snap.apply(headers, chain)
	if err != nil {
		return nil, err
	}
	c.recents.Add(snap.Hash, snap)

	// If we've generated a new checkpoint snapshot, save to disk
	if snap.Number%checkpointInterval == 0 && len(headers) > 0 {
		if err = snap.store(c.db); err != nil {
			return nil, err
		}
		log.Trace("Stored snapshot to disk", "number", snap.Number, "hash", snap.Hash)
	}
	return snap, err
}

// VerifyUncles implements consensus.Engine, always returning an error for any
// uncles as this consensus mechanism doesn't permit uncles.
func (c *Oasys) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}
	return nil
}

// verifySeal checks whether the signature contained in the header satisfies the
// consensus protocol requirements. The method accepts an optional list of parent
// headers that aren't yet part of the local blockchain to generate the snapshots
// from.
func (c *Oasys) verifySeal(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header, scheduler *scheduler) error {
	// Verifying the genesis block is not supported
	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}

	// Resolve the authorization key and check against validators
	validator, err := ecrecover(header, c.signatures)
	if err != nil {
		return err
	}
	if validator != header.Coinbase {
		return errCoinBaseMisMatch
	}
	if !scheduler.exists(validator) {
		return errUnauthorizedValidator
	}

	// Ensure that the difficulty corresponds to the turn-ness of the validator
	if !c.fakeDiff {
		difficulty := scheduler.difficulty(number, validator, c.chainConfig.IsForkedOasysExtendDifficulty(header.Number))
		if header.Difficulty.Cmp(difficulty) != 0 {
			return errWrongDifficulty
		}
	}
	return nil
}

// Prepare implements consensus.Engine, preparing all the consensus fields of the
// header for running the transactions on top.
func (c *Oasys) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	number := header.Number.Uint64()
	// header.Coinbase = c.signer
	header.Nonce = types.BlockNonce{}

	// Mix digest is reserved for now, set to empty
	header.MixDigest = common.Hash{}

	env, _ := getEnvironmentValue(c.chainConfig, number)

	// Ensure the extra data has all its components
	if len(header.Extra) < extraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, extraVanity-len(header.Extra))...)
	}
	header.Extra = header.Extra[:extraVanity]

	var (
		validators []common.Address
		stakes     []*big.Int
	)
	if number > 0 && env.IsEpochStartBlock(number) {
		result, err := getNextValidators(c.chainConfig, c.ethAPI, header.ParentHash, env.Epoch(number), number)
		if err != nil {
			log.Error("Failed to get validators", "in", "Prepare", "hash", header.ParentHash, "number", number, "err", err)
			return err
		}
		validators, stakes = result.Operators, result.Stakes
		header.Extra = append(header.Extra, c.getExtraHeaderValueInEpoch(header.Number, validators)...)
	} else {
		snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
		if err != nil {
			return err
		}
		validators, stakes = snap.validatorsToTuple()
	}
	scheduler, err := c.scheduler(chain, header, env, validators, stakes)
	if err != nil {
		log.Error("Failed to get scheduler", "in", "Prepare", "number", number, "err", err)
		return err
	}

	// Add extra seal
	header.Extra = append(header.Extra, make([]byte, extraSeal)...)

	// Add the difficulty
	header.Difficulty = scheduler.difficulty(number, c.signer, c.chainConfig.IsForkedOasysExtendDifficulty(header.Number))

	// Ensure the timestamp has the correct delay
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Time = parent.Time + env.BlockPeriod.Uint64() + scheduler.backOffTime(number, c.signer)
	if header.Time < uint64(time.Now().Unix()) {
		header.Time = uint64(time.Now().Unix())
	}

	return nil
}

// Finalize implements consensus.Engine, ensuring no uncles are set, nor block
// rewards given.
func (c *Oasys) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs *[]*types.Transaction,
	uncles []*types.Header, withdrawals []*types.Withdrawal, receipts *[]*types.Receipt, systemTxs *[]*types.Transaction, usedGas *uint64) error {
	if len(withdrawals) > 0 {
		return errors.New("oasys does not support withdrawals")
	}

	if err := verifyTx(header, *txs); err != nil {
		return err
	}

	hash := header.Hash()
	number := header.Number.Uint64()
	env, nextEnv := getEnvironmentValue(c.chainConfig, number)

	cx := chainContext{Chain: chain, oasys: c}
	if number == 1 {
		err := c.initializeSystemContracts(state, header, cx, txs, receipts, systemTxs, usedGas, false)
		if err != nil {
			log.Error("Failed to initialize system contracts", "in", "Finalize", "hash", hash, "number", number, "err", err)
			return err
		}
		log.Info("Initialized system contracts", "in", "Finalize", "hash", hash, "number", number)
	}
	if nextEnv != nil && env.ShouldUpdate(nextEnv, number) {
		err := c.updateEnvironmentValue(nextEnv, state, header, cx, txs, receipts, systemTxs, usedGas, false)
		if err != nil {
			log.Error("Failed to update environment value", "in", "Finalize", "hash", hash, "number", number, "err", err)
			return err
		}
		log.Info("Updated environment value", "in", "Finalize", "hash", hash, "number", number)
	}

	var (
		validators []common.Address
		stakes     []*big.Int
	)
	if env.IsEpochStartBlock(number) {
		result, err := getNextValidators(c.chainConfig, c.ethAPI, header.ParentHash, env.Epoch(number), number)
		if err != nil {
			log.Error("Failed to get validators", "in", "Finalize", "hash", header.ParentHash, "number", number, "err", err)
			return err
		}
		validators, stakes = result.Operators, result.Stakes
	} else {
		snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
		if err != nil {
			return err
		}
		validators, stakes = snap.validatorsToTuple()
	}
	scheduler, err := c.scheduler(chain, header, env, validators, stakes)
	if err != nil {
		log.Error("Failed to get scheduler", "in", "Finalize", "number", number, "err", err)
		return err
	}

	if err := c.addBalanceToStakeManager(state, header.ParentHash, number, env); err != nil {
		log.Error("Failed to add balance to staking contract", "in", "Finalize", "hash", header.ParentHash, "number", number, "err", err)
		return err
	}

	if env.IsEpochStartBlock(number) {
		// If the block is a epoch block, verify the validator list or hash
		actual := header.Extra[extraVanity : len(header.Extra)-extraSeal]
		if err := c.verifyExtraHeaderValueInEpoch(header.Number, actual, validators); err != nil {
			return err
		}
	}

	if number >= c.config.Epoch {
		validator, err := ecrecover(header, c.signatures)
		if err != nil {
			return err
		}
		if expected := *scheduler.expect(number); expected != validator {
			if err := c.slash(expected, scheduler.schedules(), state, header, cx, txs, receipts, systemTxs, usedGas, false); err != nil {
				log.Error("Failed to slash validator", "in", "Finalize", "hash", hash, "number", number, "address", expected, "err", err)
			}
		}
	}

	if len(*systemTxs) > 0 {
		return errors.New("must not contain system transactions")
	}

	return nil
}

// FinalizeAndAssemble implements consensus.Engine, ensuring no uncles are set,
// nor block rewards given, and returns the final block.
func (c *Oasys) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
	uncles []*types.Header, receipts []*types.Receipt, withdrawals []*types.Withdrawal) (*types.Block, []*types.Receipt, error) {
	if txs == nil {
		txs = make([]*types.Transaction, 0)
	}
	if receipts == nil {
		receipts = make([]*types.Receipt, 0)
	}

	if len(withdrawals) > 0 {
		return nil, receipts, errors.New("oasys does not support withdrawals")
	}
	if err := verifyTx(header, txs); err != nil {
		return nil, nil, err
	}

	hash := header.Hash()
	number := header.Number.Uint64()
	env, nextEnv := getEnvironmentValue(c.chainConfig, number)

	cx := chainContext{Chain: chain, oasys: c}
	if number == 1 {
		err := c.initializeSystemContracts(state, header, cx, &txs, &receipts, nil, &header.GasUsed, true)
		if err != nil {
			log.Error("Failed to initialize system contracts", "in", "FinalizeAndAssemble", "hash", hash, "err", err)
			return nil, nil, err
		}
		log.Info("Initialized system contracts", "in", "FinalizeAndAssemble", "hash", hash, "number", number)
	}
	if nextEnv != nil && env.ShouldUpdate(nextEnv, number) {
		err := c.updateEnvironmentValue(nextEnv, state, header, cx, &txs, &receipts, nil, &header.GasUsed, true)
		if err != nil {
			log.Error("Failed to update environment value", "in", "FinalizeAndAssemble", "hash", hash, "err", err)
			return nil, nil, err
		}
		log.Info("Updated environment value", "in", "FinalizeAndAssemble", "hash", hash, "number", number)
	}

	var (
		validators []common.Address
		stakes     []*big.Int
	)
	if env.IsEpochStartBlock(number) {
		result, err := getNextValidators(c.chainConfig, c.ethAPI, header.ParentHash, env.Epoch(number), number)
		if err != nil {
			log.Error("Failed to get validators", "in", "FinalizeAndAssemble", "hash", header.ParentHash, "number", number, "err", err)
			return nil, nil, err
		}
		validators, stakes = result.Operators, result.Stakes
	} else {
		snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
		if err != nil {
			return nil, nil, err
		}
		validators, stakes = snap.validatorsToTuple()
	}
	scheduler, err := c.scheduler(chain, header, env, validators, stakes)
	if err != nil {
		log.Error("Failed to get scheduler", "in", "FinalizeAndAssemble", "number", number, "err", err)
		return nil, nil, err
	}

	if err := c.addBalanceToStakeManager(state, header.ParentHash, number, env); err != nil {
		log.Error("Failed to add balance to staking contract", "in", "FinalizeAndAssemble", "hash", hash, "number", number, "err", err)
		return nil, nil, err
	}

	if number >= c.config.Epoch {
		if expected := *scheduler.expect(number); expected != header.Coinbase {
			if err := c.slash(expected, scheduler.schedules(), state, header, cx, &txs, &receipts, nil, &header.GasUsed, true); err != nil {
				log.Error("Failed to slash validator", "in", "FinalizeAndAssemble", "hash", hash, "number", number, "address", expected, "err", err)
			}
		}
	}

	if header.GasLimit < header.GasUsed {
		return nil, nil, errors.New("gas consumption of system txs exceed the gas limit")
	}

	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	header.UncleHash = types.CalcUncleHash(nil)
	return types.NewBlock(header, txs, nil, receipts, trie.NewStackTrie(nil)), receipts, nil
}

// Authorize injects a private key into the consensus engine to mint new blocks
// with.
func (c *Oasys) Authorize(signer common.Address, signFn SignerFn, txSignFn TxSignerFn) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.signer = signer
	c.signFn = signFn
	c.txSignFn = txSignFn
}

// Seal implements consensus.Engine, attempting to create a sealed block using
// the local signing credentials.
func (c *Oasys) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := block.Header()

	// Sealing the genesis block is not supported
	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}
	// For 0-period chains, refuse to seal empty blocks (no reward but would spin sealing)
	if c.config.Period == 0 && len(block.Transactions()) == 0 {
		return errors.New("sealing paused while waiting for transactions")
	}
	// Don't hold the signer fields for the entire sealing procedure
	c.lock.RLock()
	validator, signFn := c.signer, c.signFn
	c.lock.RUnlock()

	env, _ := getEnvironmentValue(c.chainConfig, number)

	// Bail out if we're unauthorized to sign a block
	var exists bool
	if number > 0 && env.IsEpochStartBlock(number) {
		result, err := getNextValidators(c.chainConfig, c.ethAPI, header.ParentHash, env.Epoch(number), number)
		if err != nil {
			log.Error("Failed to get validators", "in", "Seal", "hash", header.ParentHash, "number", number, "err", err)
			return err
		}
		exists = result.Exists(validator)
	} else {
		snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
		if err != nil {
			return err
		}
		exists = snap.exists(validator)
	}

	if !exists {
		return errUnauthorizedValidator
	}

	// Sweet, the protocol permits us to sign the block, wait for our time
	delay := time.Until(time.Unix(int64(header.Time), 0))
	// Sign all the things!
	sighash, err := signFn(accounts.Account{Address: validator}, accounts.MimetypeOasys, OasysRLP(header))
	if err != nil {
		return err
	}
	copy(header.Extra[len(header.Extra)-extraSeal:], sighash)
	// Wait until sealing is terminated or delay timeout.
	log.Trace("Waiting for slot to sign and propagate", "delay", common.PrettyDuration(delay))
	go func() {
		select {
		case <-stop:
			return
		case <-time.After(delay):
		}

		select {
		case results <- block.WithSeal(header):
		default:
			log.Warn("Sealing result is not read by miner", "sealhash", SealHash(header))
		}
	}()

	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have:
func (c *Oasys) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	number := parent.Number.Uint64()
	env, _ := getEnvironmentValue(c.chainConfig, number)

	var (
		validators []common.Address
		stakes     []*big.Int
	)
	if env.IsEpochStartBlock(number) {
		result, err := getNextValidators(c.chainConfig, c.ethAPI, parent.Hash(), env.Epoch(number), number)
		if err != nil {
			log.Error("Failed to get validators", "in", "Seal", "hash", parent.Hash(), "number", number, "err", err)
			return nil
		}
		validators, stakes = result.Operators, result.Stakes
	} else {
		snap, err := c.snapshot(chain, number, parent.Hash(), nil)
		if err != nil {
			return nil
		}
		validators, stakes = snap.validatorsToTuple()
	}
	scheduler, err := c.scheduler(chain, parent, env, validators, stakes)
	if err != nil {
		log.Error("Failed to get scheduler", "in", "CalcDifficulty", "number", number, "err", err)
		return nil
	}

	return scheduler.difficulty(number, c.signer, c.chainConfig.IsForkedOasysExtendDifficulty(parent.Number))
}

// SealHash returns the hash of a block prior to it being sealed.
func (c *Oasys) SealHash(header *types.Header) common.Hash {
	return SealHash(header)
}

// Close implements consensus.Engine. It's a noop for oasys as there are no background threads.
func (c *Oasys) Close() error {
	return nil
}

// APIs implements consensus.Engine, returning the user facing RPC API to allow
// controlling the signer voting.
func (c *Oasys) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return []rpc.API{{
		Namespace: "oasys",
		Version:   "1.0",
		Service:   &API{chain: chain, oasys: c},
		Public:    false,
	}}
}

// Converting the validator list for the extra header field.
func (c *Oasys) getExtraHeaderValueInEpoch(number *big.Int, validators []common.Address) []byte {
	cpy := make([]common.Address, len(validators))
	copy(cpy, validators)

	forked := c.chainConfig.IsForkedOasysPublication(number)
	if !forked {
		sort.Sort(validatorsAscending(cpy))
	}

	extra := make([]byte, len(cpy)*common.AddressLength)
	for i, v := range cpy {
		copy(extra[i*common.AddressLength:], v.Bytes())
	}

	// Convert to hash because there may be many validators.
	if forked {
		extra = crypto.Keccak256(extra)
	}
	return extra
}

// Verify the length of the Extra header field.
func (c *Oasys) verifyExtraHeaderLengthInEpoch(number *big.Int, length int) error {
	if c.chainConfig.IsForkedOasysPublication(number) {
		if length != crypto.DigestLength {
			return errInvalidEpochHash
		}
	} else if length%common.AddressLength != 0 {
		return errInvalidCheckpointValidators
	}
	return nil
}

// Verify the value of the Extra header field.
func (c *Oasys) verifyExtraHeaderValueInEpoch(number *big.Int, actual []byte, validators []common.Address) error {
	expect := c.getExtraHeaderValueInEpoch(number, validators)
	if bytes.Equal(actual, expect) {
		return nil
	}

	if c.chainConfig.IsForkedOasysPublication(number) {
		return errMismatchingEpochHash
	}
	return errMismatchingEpochValidators
}

// SealHash returns the hash of a block prior to it being sealed.
func SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	encodeSigHeader(hasher, header)
	hasher.(crypto.KeccakState).Read(hash[:])
	return hash
}

// OasysRLP returns the rlp bytes which needs to be signed for the proof-of-stake
// sealing. The RLP to sign consists of the entire header apart from the 65 byte signature
// contained at the end of the extra data.
//
// Note, the method requires the extra data to be at least 65 bytes, otherwise it
// panics. This is done to avoid accidentally using both forms (signature present
// or not), which could be abused to produce different hashes for the same header.
func OasysRLP(header *types.Header) []byte {
	b := new(bytes.Buffer)
	encodeSigHeader(b, header)
	return b.Bytes()
}

func encodeSigHeader(w io.Writer, header *types.Header) {
	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra[:len(header.Extra)-crypto.SignatureLength], // Yes, this will panic if extra is too short
		header.MixDigest,
		header.Nonce,
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	if header.WithdrawalsHash != nil {
		panic("unexpected withdrawal hash value in oasys")
	}
	if header.ExcessBlobGas != nil {
		panic("unexpected excess blob gas value in oasys")
	}
	if header.BlobGasUsed != nil {
		panic("unexpected blob gas used value in oasys")
	}
	if header.ParentBeaconRoot != nil {
		panic("unexpected parent beacon root value in oasys")
	}
	if err := rlp.Encode(w, enc); err != nil {
		panic("can't encode: " + err.Error())
	}
}

func (c *Oasys) addBalanceToStakeManager(state *state.StateDB, hash common.Hash, number uint64, env *environmentValue) error {
	if !env.IsEpochStartBlock(number) || env.Epoch(number) < 3 || env.Epoch(number) > 60 {
		return nil
	}

	var (
		rewards *big.Int
		err     error
	)

	if rewards, err = getRewards(c.ethAPI, hash); err != nil {
		log.Error("Failed to get rewards", "hash", hash, "err", err)
		return err
	}
	if rewards.Cmp(common.Big0) == 0 {
		return nil
	}

	state.AddBalance(stakeManager.address, rewards)
	log.Info("Balance added to stake manager", "hash", hash, "amount", rewards.String())
	return nil
}

func (c *Oasys) scheduler(chain consensus.ChainHeaderReader, header *types.Header,
	env *environmentValue, validators []common.Address, stakes []*big.Int) (*scheduler, error) {
	number := header.Number.Uint64()

	// Previous epoch does not exists.
	if number < c.config.Epoch {
		return newScheduler(env, 0, newWeightedChooser(validators, stakes, 0)), nil
	}

	// After the second epoch, the hash of the last block
	// of the previous epoch is used as the random seed.
	seedHash, err := getPrevEpochLastBlockHash(c.config, chain, env, header)
	if err != nil {
		return nil, err
	} else if seedHash == emptyHash {
		return nil, errors.New("invalid seed hash")
	}

	if cache, ok := schedulerCache.Get(seedHash); ok {
		return cache.(*scheduler), nil
	}

	var seed int64
	if env.Epoch(number) >= c.chainConfig.OasysShortenedBlockTimeStartEpoch().Uint64() {
		seed = new(big.Int).Mod(seedHash.Big(), BigMaxInt64).Int64()
	} else {
		seed = seedHash.Big().Int64()
	}

	created := newScheduler(env, env.EpochStartBlock(number),
		newWeightedChooser(validators, stakes, seed))
	schedulerCache.Add(seedHash, created)
	return created, nil
}

// Oasys transaction verification
func verifyTx(header *types.Header, txs []*types.Transaction) error {
	for _, tx := range txs {
		if err := core.VerifyTx(tx); err != nil {
			return err
		}
	}
	return nil
}
