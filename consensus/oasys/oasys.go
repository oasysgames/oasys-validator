package oasys

import (
	"bytes"
	"errors"
	"fmt"
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
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	lru "github.com/hashicorp/golang-lru"
	"github.com/prysmaticlabs/prysm/v5/crypto/bls"
	"github.com/willf/bitset"
	"golang.org/x/crypto/sha3"
)

const (
	checkpointInterval = 1024 // Number of blocks after which to save the vote snapshot to the database
	inmemorySnapshots  = 128  // Number of recent vote snapshots to keep in memory
	inmemorySignatures = 4096 // Number of recent block signatures to keep in memory

	extraVanity = 32                     // Fixed number of extra-data prefix bytes reserved for signer vanity
	extraSeal   = crypto.SignatureLength // Fixed number of extra-data suffix bytes reserved for signer seal

	envValuesLen          = 32 * 9
	addressBytesLen       = common.AddressLength
	stakeBytesLen         = 32
	validatorInfoBytesLen = addressBytesLen*2 + stakeBytesLen + types.BLSPublicKeyLength
	validatorNumberSize   = 1 // Fixed number of extra prefix bytes reserved for validator number

	backoffWiggleTime = uint64(1) // second
)

// Oasys proof-of-stake protocol constants.
var (
	epochLength = uint64(30000) // Default number of blocks after which to checkpoint and reset the pending votes

	uncleHash = types.CalcUncleHash(nil) // Always Keccak256(RLP([])) as uncles are meaningless outside of PoW.

	diffInTurn = big.NewInt(2) // Block difficulty for in-turn signatures
	diffNoTurn = big.NewInt(1) // Block difficulty for out-of-turn signatures

	bigMaxInt64 = big.NewInt(math.MaxInt64)
	ether       = big.NewInt(1_000_000_000_000_000_000)
	totalSupply = new(big.Int).Mul(big.NewInt(10_000_000_000), ether) // From WhitePaper

	verifyVoteAttestationErrorCounter = metrics.NewRegisteredCounter("oasys/verifyVoteAttestation/error", nil)
	updateAttestationErrorCounter     = metrics.NewRegisteredCounter("oasys/updateAttestation/error", nil)
	validVotesfromSelfCounter         = metrics.NewRegisteredCounter("oasys/VerifyVote/self", nil)
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

	// errNoEnvironmentValue is returned if the extra data does not contain the environment value
	errNoEnvironmentValue = errors.New("no environment value in the extra data")
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
	pubkey, err := crypto.Ecrecover(types.SealHash(header).Bytes(), signature)
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
	VotePool consensus.VotePool
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
	// Apply parent headers to the snapshot, the snapshot updates is only processed here.
	snap, err := c.snapshot(chain, number-1, header.ParentHash, parents)
	if err != nil {
		return fmt.Errorf("failed to retrieve snapshot, in: verifyHeader, blockNumber: %d, parentHash: %x, err: %v", number, header.ParentHash, err)
	}
	// Get the environment value from snapshot except the block is epoch block
	env, err := c.environment(chain, header, snap, true)
	if err != nil {
		return fmt.Errorf("failed to get environment value, in: verifyHeader, err: %v", err)
	}
	// Ensure that the extra-data contains extra data aside from the vanity and seal
	extraLenExceptVanityAndSeal := len(header.Extra) - extraVanity - extraSeal
	if env.IsEpoch(number) {
		if err := c.verifyExtraHeaderLengthInEpoch(header.Number, extraLenExceptVanityAndSeal); err != nil {
			return err
		}
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
	return c.verifyCascadingFields(chain, header, parents, snap, env)
}

// verifyCascadingFields verifies all the header fields that are not standalone,
// rather depend on a batch of previous headers. The caller may optionally pass
// in a batch of parents (ascending order) to avoid looking those up from the
// database. This is useful for concurrently verifying a batch of new headers.
func (c *Oasys) verifyCascadingFields(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header, snap *Snapshot, env *params.EnvironmentValue) error {
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

	// Get the validators from snapshot except the block is epoch block
	validators, err := c.getNextValidators(chain, header, snap, true)
	if err != nil {
		return fmt.Errorf("failed to get validators, in: verifyCascadingFields, err: %v", err)
	}
	// Ensure that the block's timestamp is older than the scheduled validator backoff time
	scheduler, err := c.scheduler(chain, header, env, validators.Operators, validators.Stakes)
	if err != nil {
		return fmt.Errorf("failed to get scheduler. blockNumber: %d, in: verifyCascadingFields, err: %v", number, err)
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

	// Verify vote attestation for fast finality.
	if err := c.verifyVoteAttestation(chain, header, parents, env, validators); err != nil {
		log.Warn("Verify vote attestation failed", "error", err, "hash", header.Hash(), "number", header.Number,
			"parent", header.ParentHash, "coinbase", header.Coinbase, "extra", common.Bytes2Hex(header.Extra))
		verifyVoteAttestationErrorCounter.Inc(1)
		if chain.Config().IsFastFinalityEnabled(header.Number) {
			return err
		}
	}

	// All basic checks passed, verify the seal and return
	return c.verifySeal(chain, header, parents, scheduler)
}

// getParent returns the parent of a given block.
func (o *Oasys) getParent(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) (*types.Header, error) {
	var parent *types.Header
	number := header.Number.Uint64()
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, number-1)
	}

	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return nil, consensus.ErrUnknownAncestor
	}
	return parent, nil
}

// verifyVoteAttestation checks whether the vote attestation in the header is valid.
func (o *Oasys) verifyVoteAttestation(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header, env *params.EnvironmentValue, validators *nextValidators) error {
	attestation, err := getVoteAttestationFromHeader(header, o.chainConfig, o.config, env)
	if err != nil {
		return err
	}
	if attestation == nil {
		return nil
	}
	if attestation.Data == nil {
		return errors.New("invalid attestation, vote data is nil")
	}
	if len(attestation.Extra) > types.MaxAttestationExtraLength {
		return fmt.Errorf("invalid attestation, too large extra length: %d", len(attestation.Extra))
	}

	// Get parent block
	parent, err := o.getParent(chain, header, parents)
	if err != nil {
		return err
	}

	// The target block should be direct parent.
	targetNumber := attestation.Data.TargetNumber
	targetHash := attestation.Data.TargetHash
	if targetNumber != parent.Number.Uint64() || targetHash != parent.Hash() {
		return fmt.Errorf("invalid attestation, target mismatch, expected block: %d, hash: %s; real block: %d, hash: %s",
			parent.Number.Uint64(), parent.Hash(), targetNumber, targetHash)
	}

	// The source block should be the highest justified block.
	sourceNumber := attestation.Data.SourceNumber
	sourceHash := attestation.Data.SourceHash
	headers := []*types.Header{parent}
	if len(parents) > 0 {
		headers = parents
	}
	justifiedBlockNumber, justifiedBlockHash, err := o.GetJustifiedNumberAndHash(chain, headers)
	if err != nil {
		return errors.New("unexpected error when getting the highest justified number and hash")
	}
	if sourceNumber != justifiedBlockNumber || sourceHash != justifiedBlockHash {
		return fmt.Errorf("invalid attestation, source mismatch, expected block: %d, hash: %s; real block: %d, hash: %s",
			justifiedBlockNumber, justifiedBlockHash, sourceNumber, sourceHash)
	}

	// Filter out valid validator from attestation.
	validatorsBitSet := bitset.From([]uint64{uint64(attestation.VoteAddressSet)})
	if validatorsBitSet.Count() > uint(len(validators.Operators)) {
		return fmt.Errorf("invalid attestation, vote number(=%d) larger than validators number(=%d)", validatorsBitSet.Count(), len(validators.Operators))
	}
	votedAddrs := make([]types.BLSPublicKey, 0, validatorsBitSet.Count())
	votedPubKeys := make([]bls.PublicKey, 0, validatorsBitSet.Count())
	for i := range validators.Operators {
		voterIndex := i + 1
		if !validatorsBitSet.Test(uint(voterIndex)) {
			continue
		}
		votedAddrs = append(votedAddrs, validators.VoteAddresses[i])
		votedPubKey, err := bls.PublicKeyFromBytes(validators.VoteAddresses[i][:])
		if err != nil {
			return fmt.Errorf("BLS public key converts failed: %v", err)
		}
		votedPubKeys = append(votedPubKeys, votedPubKey)
	}

	// The valid voted validators should be no less than 2/3 voting power.
	if !isSufficientVotes(votedAddrs, validators) {
		return errors.New("invalid attestation, not enough voting power voted")
	}

	// Verify the aggregated signature.
	aggSig, err := bls.SignatureFromBytes(attestation.AggSignature[:])
	if err != nil {
		return fmt.Errorf("BLS signature converts failed: %v", err)
	}
	if !aggSig.FastAggregateVerify(votedPubKeys, attestation.Data.Hash()) {
		return errors.New("invalid attestation, signature verify failed")
	}

	return nil
}

func isSufficientVotes(votedAddrs []types.BLSPublicKey, validators *nextValidators) bool {
	totalStake := big.NewInt(0)
	voterTotalStake := big.NewInt(0)
	for i, stake := range validators.Stakes {
		totalStake.Add(totalStake, stake)
		for _, voteAddr := range votedAddrs {
			if voteAddr == validators.VoteAddresses[i] {
				voterTotalStake.Add(voterTotalStake, stake)
				break
			}
		}
	}
	// the voter's total stake should be greater than 2/3 of the total stake
	threshold := new(big.Int).Mul(totalStake, big.NewInt(2))
	threshold.Div(threshold, big.NewInt(3))
	return voterTotalStake.Cmp(threshold) >= 0
}

// getValidatorsFromHeader returns the next validators extracted from the header's extra field if exists.
// The validators bytes would be contained only in the epoch block's header, and its each validator bytes length is fixed.
// Layout: |--Extra Vanity--|--EnvironmentValue--| --Validator Number--|--Owner(or Empty)--|--Operator(or Empty)--|---Stake(or Empty)--|--Vote Address(or Empty)--|--Vote Attestation(or Empty)--|--Extra Seal--|
func getValidatorsFromHeader(header *types.Header) (*nextValidators, error) {
	if len(header.Extra) <= extraVanity+extraSeal {
		return nil, fmt.Errorf("no validators in the extra data, extra length: %d", len(header.Extra))
	}

	num := int(header.Extra[extraVanity+envValuesLen])
	lenNoAttestation := extraVanity + envValuesLen + validatorNumberSize + num*validatorInfoBytesLen
	if num == 0 || len(header.Extra) < lenNoAttestation {
		return nil, fmt.Errorf("missing validator info in the extra data, extra length: %d", len(header.Extra))
	}

	vals := &nextValidators{
		Owners:        make([]common.Address, num),
		Operators:     make([]common.Address, num),
		Stakes:        make([]*big.Int, num),
		VoteAddresses: make([]types.BLSPublicKey, num),
	}
	start := extraVanity + envValuesLen + validatorNumberSize
	for i := 0; i < num; i++ {
		copy(vals.Owners[i][:], header.Extra[start:start+addressBytesLen])
		start += addressBytesLen
		copy(vals.Operators[i][:], header.Extra[start:start+addressBytesLen])
		start += addressBytesLen
		vals.Stakes[i] = new(big.Int).SetBytes(header.Extra[start : start+stakeBytesLen])
		start += stakeBytesLen
		copy(vals.VoteAddresses[i][:], header.Extra[start:start+types.BLSPublicKeyLength])
		start += types.BLSPublicKeyLength
	}

	return vals, nil
}

func getEnvironmentFromHeader(header *types.Header) (*params.EnvironmentValue, error) {
	// As the vote attestation length is not determistically fixed, we omit the vote attestation info
	// even if the vote attestations are included, the length is enough smaller than the environment value
	if len(header.Extra) < extraVanity+envValuesLen+extraSeal {
		return nil, errNoEnvironmentValue
	}

	start := extraVanity
	env := &params.EnvironmentValue{
		StartBlock:         new(big.Int).SetBytes(header.Extra[start : start+32]),
		StartEpoch:         new(big.Int).SetBytes(header.Extra[start+32 : start+64]),
		BlockPeriod:        new(big.Int).SetBytes(header.Extra[start+64 : start+96]),
		EpochPeriod:        new(big.Int).SetBytes(header.Extra[start+96 : start+128]),
		RewardRate:         new(big.Int).SetBytes(header.Extra[start+128 : start+160]),
		CommissionRate:     new(big.Int).SetBytes(header.Extra[start+160 : start+192]),
		ValidatorThreshold: new(big.Int).SetBytes(header.Extra[start+192 : start+224]),
		JailThreshold:      new(big.Int).SetBytes(header.Extra[start+224 : start+256]),
		JailPeriod:         new(big.Int).SetBytes(header.Extra[start+256 : start+288]),
	}
	return env, nil
}

// getVoteAttestationFromHeader returns the vote attestation extracted from the header's extra field if exists.
func getVoteAttestationFromHeader(header *types.Header, chainConfig *params.ChainConfig, oasysConfig *params.OasysConfig, env *params.EnvironmentValue) (*types.VoteAttestation, error) {
	if len(header.Extra) <= extraVanity+extraSeal {
		return nil, nil
	}

	if !chainConfig.IsFastFinalityEnabled(header.Number) {
		return nil, nil
	}

	var attestationBytes []byte
	if env.IsEpoch(header.Number.Uint64()) {
		num := int(header.Extra[extraVanity+envValuesLen])
		if len(header.Extra) <= extraVanity+extraSeal+validatorNumberSize+num*validatorInfoBytesLen {
			return nil, nil
		}
		start := extraVanity + envValuesLen + validatorNumberSize + num*validatorInfoBytesLen
		end := len(header.Extra) - extraSeal
		attestationBytes = header.Extra[start:end]
	} else {
		attestationBytes = header.Extra[extraVanity : len(header.Extra)-extraSeal]
	}

	// exit if no attestation info
	if len(attestationBytes) == 0 {
		return nil, nil
	}

	var attestation types.VoteAttestation
	if err := rlp.Decode(bytes.NewReader(attestationBytes), &attestation); err != nil {
		return nil, fmt.Errorf("block %d has vote attestation info, decode err: %s", header.Number.Uint64(), err)
	}
	return &attestation, nil
}

// snapshot retrieves the authorization snapshot at a given point in time.
// !!! be careful
// the block with `number` and `hash` is just the last element of `parents`,
// unlike other interfaces such as verifyCascadingFields, `parents` are real parents
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

				snap = newSnapshot(c.chainConfig, c.signatures, c.ethAPI,
					number, hash, validators, params.InitialEnvironmentValue(c.config))
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
	snap, err := snap.apply(headers, chain, c.config)
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

func assembleValidators(validators *nextValidators) []byte {
	extra := make([]byte, 0, 1+len(validators.Operators)*common.AddressLength)
	// add validator number
	extra = append(extra, byte(len(validators.Operators)))
	// add validator info
	for i := 0; i < len(validators.Operators); i++ {
		extra = append(extra, validators.Owners[i].Bytes()...)
		extra = append(extra, validators.Operators[i].Bytes()...)
		extra = append(extra, bigTo32BytesLeftPadding(validators.Stakes[i])...)
		extra = append(extra, validators.VoteAddresses[i][:]...)
	}
	return extra
}

func assembleEnvironmentValue(env *params.EnvironmentValue) []byte {
	extra := make([]byte, 0, envValuesLen)
	extra = append(extra, bigTo32BytesLeftPadding(env.StartBlock)...)
	extra = append(extra, bigTo32BytesLeftPadding(env.StartEpoch)...)
	extra = append(extra, bigTo32BytesLeftPadding(env.BlockPeriod)...)
	extra = append(extra, bigTo32BytesLeftPadding(env.EpochPeriod)...)
	extra = append(extra, bigTo32BytesLeftPadding(env.RewardRate)...)
	extra = append(extra, bigTo32BytesLeftPadding(env.CommissionRate)...)
	extra = append(extra, bigTo32BytesLeftPadding(env.ValidatorThreshold)...)
	extra = append(extra, bigTo32BytesLeftPadding(env.JailThreshold)...)
	extra = append(extra, bigTo32BytesLeftPadding(env.JailPeriod)...)
	return extra
}

func (c *Oasys) assembleVoteAttestation(chain consensus.ChainHeaderReader, header *types.Header, validators *nextValidators) error {
	if !c.chainConfig.IsFastFinalityEnabled(header.Number) || header.Number.Uint64() < 2 {
		return nil
	}

	if c.VotePool == nil {
		return nil
	}

	// Fetch direct parent's votes
	parent := chain.GetHeaderByHash(header.ParentHash)
	if parent == nil {
		return errors.New("parent not found")
	}
	votes := c.VotePool.FetchVoteByBlockHash(parent.Hash())

	if len(votes) == 0 {
		log.Debug("no votes found, skip assemble vote attestation", "header", header.Hash(), "number", header.Number, "parent", parent.Hash())
		return nil
	}

	// Check if the number of votes is sufficient
	votedAddrs := make([]types.BLSPublicKey, 0, len(votes))
	for _, vote := range votes {
		votedAddrs = append(votedAddrs, vote.VoteAddress)
	}
	if !isSufficientVotes(votedAddrs, validators) {
		log.Debug("vote number less than 2/3 voting power, skip assemble vote attestation", "header", header.Hash(), "number", header.Number, "parent", parent.Hash(), "votes", len(votes))
		return nil
	}

	// Prepare vote attestation
	// Prepare vote data
	justifiedBlockNumber, justifiedBlockHash, err := c.GetJustifiedNumberAndHash(chain, []*types.Header{parent})
	if err != nil {
		return errors.New("unexpected error when getting the highest justified number and hash")
	}
	attestation := &types.VoteAttestation{
		Data: &types.VoteData{
			SourceNumber: justifiedBlockNumber,
			SourceHash:   justifiedBlockHash,
			TargetNumber: parent.Number.Uint64(),
			TargetHash:   parent.Hash(),
		},
	}
	// Check vote data from votes
	for _, vote := range votes {
		if vote.Data.Hash() != attestation.Data.Hash() {
			return fmt.Errorf("vote check error, expected: %v, real: %v", attestation.Data, vote)
		}
	}
	// Prepare aggregated vote signature
	voteAddrSet := make(map[types.BLSPublicKey]struct{}, len(votes))
	signatures := make([][]byte, 0, len(votes))
	for _, vote := range votes {
		voteAddrSet[vote.VoteAddress] = struct{}{}
		signatures = append(signatures, vote.Signature[:])
	}
	sigs, err := bls.MultipleSignaturesFromBytes(signatures)
	if err != nil {
		return err
	}
	copy(attestation.AggSignature[:], bls.AggregateSignatures(sigs).Marshal())
	// Prepare vote address bitset.
	for i, voteAddr := range validators.VoteAddresses {
		if _, ok := voteAddrSet[voteAddr]; ok {
			voterIndex := i + 1
			attestation.VoteAddressSet |= 1 << voterIndex
		}
	}
	validatorsBitSet := bitset.From([]uint64{uint64(attestation.VoteAddressSet)})
	if validatorsBitSet.Count() < uint(len(signatures)) {
		log.Warn(fmt.Sprintf("assembleVoteAttestation, check VoteAddress Set failed, expected:%d, real:%d", len(signatures), validatorsBitSet.Count()))
		return errors.New("invalid attestation, check VoteAddress Set failed")
	}

	// Append attestation to header extra field.
	buf := new(bytes.Buffer)
	err = rlp.Encode(buf, attestation)
	if err != nil {
		return err
	}

	// Insert vote attestation into header extra ahead extra seal.
	extraSealStart := len(header.Extra) - extraSeal
	extraSealBytes := header.Extra[extraSealStart:]
	header.Extra = append(header.Extra[0:extraSealStart], buf.Bytes()...)
	header.Extra = append(header.Extra, extraSealBytes...)

	log.Debug("successfully assemble vote attestation", "header", header.Hash(), "number", header.Number, "justifiedBlockNumber", attestation.Data.TargetNumber, "finalizeBlockNumber", attestation.Data.SourceNumber)

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

	// Ensure the extra data has all its components
	if len(header.Extra) < extraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, extraVanity-len(header.Extra))...)
	}
	header.Extra = header.Extra[:extraVanity]

	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve snapshot, in: Prepare, blockNumber: %d, parentHash: %x, err: %v", number, header.ParentHash, err)
	}
	env, err := c.environment(chain, header, snap, false)
	if err != nil {
		return fmt.Errorf("failed to get environment, in: Prepare, err: %v", err)
	}
	validators, err := c.getNextValidators(chain, header, snap, false)
	if err != nil {
		return fmt.Errorf("failed to get validators, in: Prepare, err: %v", err)
	}

	// Add the difficulty
	scheduler, err := c.scheduler(chain, header, env, validators.Operators, validators.Stakes)
	if err != nil {
		return fmt.Errorf("failed to get scheduler, in: Prepare, blockNumber: %d, err: %v", number, err)
	}
	header.Difficulty = scheduler.difficulty(number, c.signer, c.chainConfig.IsForkedOasysExtendDifficulty(header.Number))

	// Add validators to the extra data
	if env.IsEpoch(number) {
		if c.chainConfig.IsFastFinalityEnabled(header.Number) {
			header.Extra = append(header.Extra, assembleEnvironmentValue(env)...)
		}
		header.Extra = append(header.Extra, c.getExtraHeaderValueInEpoch(header.Number, validators)...)
	}

	// Add extra seal
	header.Extra = append(header.Extra, make([]byte, extraSeal)...)

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

	cx := chainContext{Chain: chain, oasys: c}
	if number == 1 {
		err := c.initializeSystemContracts(state, header, cx, txs, receipts, systemTxs, usedGas, false)
		if err != nil {
			return fmt.Errorf("failed to initialize system contracts, in: Finalize, blockNumber: %d, blockHash: %s, err: %v", number, hash, err)
		}
	}

	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve snapshot, in: Finalize, blockNumber: %d, parentHash: %x, err: %v", number, header.ParentHash, err)
	}
	env, err := c.environment(chain, header, snap, false)
	if err != nil {
		return fmt.Errorf("failed to get environment, in: Finalize, err: %v", err)
	}
	fromHeader := false // Don't retrieve validators from header, as the retrieved validators are compared with the header's validators in verifyExtraHeaderValueInEpoch
	validators, err := c.getNextValidators(chain, header, snap, fromHeader)
	if err != nil {
		return fmt.Errorf("failed to get validators, in: Finalize, err: %v", err)
	}

	// If the block is a epoch block, verify the validator list or hash
	if env.IsEpoch(number) {
		actual := header.Extra[extraVanity : len(header.Extra)-extraSeal]
		if err := c.verifyExtraHeaderValueInEpoch(header, actual, env, validators); err != nil {
			return err
		}
	}

	if err := c.addBalanceToStakeManager(state, header.ParentHash, number, env); err != nil {
		return fmt.Errorf("failed to add balance to staking contract, in: Finalize, blockNumber: %d, blockHash: %s, err: %v", number, hash, err)
	}

	if number >= c.config.Epoch {
		validator, err := ecrecover(header, c.signatures)
		if err != nil {
			return err
		}
		scheduler, err := c.scheduler(chain, header, env, validators.Operators, validators.Stakes)
		if err != nil {
			return fmt.Errorf("failed to get scheduler, in: Finalize, blockNumber: %d, blockHash: %s, err: %v", number, hash, err)
		}
		if expected := *scheduler.expect(number); expected != validator {
			if err := c.slash(expected, scheduler.schedules(), state, header, cx, txs, receipts, systemTxs, usedGas, false); err != nil {
				log.Warn("failed to slash validator", "in", "Finalize", "hash", hash, "number", number, "address", expected, "err", err)
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

	cx := chainContext{Chain: chain, oasys: c}
	if number == 1 {
		err := c.initializeSystemContracts(state, header, cx, &txs, &receipts, nil, &header.GasUsed, true)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize system contracts, in: FinalizeAndAssemble, blockNumber: %d, blockHash: %s, err: %v", number, hash, err)
		}
	}

	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve snapshot, in: FinalizeAndAssemble, blockNumber: %d, parentHash: %x, err: %v", number, header.ParentHash, err)
	}
	env, err := c.environment(chain, header, snap, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get environment, in: FinalizeAndAssemble, err: %v", err)
	}
	fromHeader := false // Not retrieve validators from header to be sure, even it's already in the header
	validators, err := c.getNextValidators(chain, header, snap, fromHeader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get validators, in: FinalizeAndAssemble, err: %v", err)
	}

	scheduler, err := c.scheduler(chain, header, env, validators.Operators, validators.Stakes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get scheduler, in: FinalizeAndAssemble, blockNumber: %d, blockHash: %s, err: %v", number, hash, err)
	}

	if err := c.addBalanceToStakeManager(state, header.ParentHash, number, env); err != nil {
		return nil, nil, fmt.Errorf("failed to add balance to staking contract, in: FinalizeAndAssemble, blockNumber: %d, blockHash: %s, err: %v", number, hash, err)
	}

	if number >= c.config.Epoch {
		if expected := *scheduler.expect(number); expected != header.Coinbase {
			if err := c.slash(expected, scheduler.schedules(), state, header, cx, &txs, &receipts, nil, &header.GasUsed, true); err != nil {
				log.Warn("failed to slash validator", "in", "FinalizeAndAssemble", "hash", hash, "number", number, "address", expected, "err", err)
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

func (c *Oasys) IsActiveValidatorAt(chain consensus.ChainHeaderReader, header *types.Header, checkVoteKeyFn func(bLSPublicKey *types.BLSPublicKey) bool) bool {
	number := header.Number.Uint64()
	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		log.Warn("failed to get snapshot", "in", "IsActiveValidatorAt", "parentHash", header.ParentHash, "number", number, "err", err)
		return false
	}
	validators, err := c.getNextValidators(chain, header, snap, true)
	if err != nil {
		log.Warn("failed to get validators", "in", "IsActiveValidatorAt", "err", err)
		return false
	}
	for i, operator := range validators.Operators {
		if operator == c.signer {
			if checkVoteKeyFn == nil || checkVoteKeyFn(&validators.VoteAddresses[i]) {
				return true
			}
		}
	}
	return false
}

// VerifyVote will verify: 1. If the vote comes from valid validators 2. If the vote's sourceNumber and sourceHash are correct
func (c *Oasys) VerifyVote(chain consensus.ChainHeaderReader, vote *types.VoteEnvelope) error {
	targetNumber := vote.Data.TargetNumber
	targetHash := vote.Data.TargetHash
	header := chain.GetHeaderByHash(targetHash)
	if header == nil {
		log.Warn("BlockHeader at current voteBlockNumber is nil", "targetNumber", targetNumber, "targetHash", targetHash)
		return errors.New("BlockHeader at current voteBlockNumber is nil")
	}
	if header.Number.Uint64() != targetNumber {
		log.Warn("unexpected target number", "expect", header.Number.Uint64(), "real", targetNumber)
		return errors.New("target number mismatch")
	}

	justifiedBlockNumber, justifiedBlockHash, err := c.GetJustifiedNumberAndHash(chain, []*types.Header{header})
	if err != nil {
		log.Warn("failed to get the highest justified number and hash", "headerNumber", header.Number, "headerHash", header.Hash())
		return errors.New("unexpected error when getting the highest justified number and hash")
	}
	if vote.Data.SourceNumber != justifiedBlockNumber || vote.Data.SourceHash != justifiedBlockHash {
		return errors.New("vote source block mismatch")
	}

	number := header.Number.Uint64()
	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return fmt.Errorf("failed to get the snapshot, in: VerifyVote, blockNumber: %d, parentHash: %s, err: %v", number, header.ParentHash, err)
	}
	validators, err := c.getNextValidators(chain, header, snap, true)
	if err != nil {
		return fmt.Errorf("failed to get the validators, in: VerifyVote, err: %v", err)
	}
	for i := range validators.Operators {
		if validators.VoteAddresses[i] == vote.VoteAddress {
			if validators.Operators[i] == c.signer {
				validVotesfromSelfCounter.Inc(1)
			}
			metrics.GetOrRegisterCounter(fmt.Sprintf("oasys/VerifyVote/%s", validators.Operators[i].String()), nil).Inc(1)
			return nil
		}
	}

	return errors.New("vote verification failed")
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

	// Bail out if we're unauthorized to sign a block
	snap, err := c.snapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve snapshot, in: Seal, blockNumber: %d, parentHash: %s, err: %v", number, header.ParentHash, err)
	}
	fromHeader := true // ok from header as the validators are assembled in `Prepare` phase
	validators, err := c.getNextValidators(chain, header, snap, fromHeader)
	if err != nil {
		return fmt.Errorf("failed to get validators, in: Seal, err: %v", err)
	}
	if !validators.Exists(validator) {
		return errUnauthorizedValidator
	}

	// Sweet, the protocol permits us to sign the block, wait for our time
	delay := time.Until(time.Unix(int64(header.Time), 0))

	// Wait until sealing is terminated or delay timeout.
	log.Trace("Waiting for slot to sign and propagate", "delay", common.PrettyDuration(delay))
	go func() {
		select {
		case <-stop:
			return
		case <-time.After(delay):
		}

		err := c.assembleVoteAttestation(chain, header, validators)
		if err != nil {
			/* If the vote attestation can't be assembled successfully, the blockchain won't get
			   fast finalized, but it can be tolerated, so just report this error here. */
			log.Error("Assemble vote attestation failed when sealing", "err", err)
		}

		// Sign all the things!
		sighash, err := signFn(accounts.Account{Address: validator}, accounts.MimetypeOasys, OasysRLP(header))
		if err != nil {
			log.Error("Sign for the block header failed when sealing", "err", err)
			return
		}
		copy(header.Extra[len(header.Extra)-extraSeal:], sighash)

		select {
		case results <- block.WithSeal(header):
		default:
			log.Warn("Sealing result is not read by miner", "sealhash", types.SealHash(header))
		}
	}()

	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have:
func (c *Oasys) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	number := parent.Number.Uint64()

	snap, err := c.snapshot(chain, number, parent.Hash(), nil)
	if err != nil {
		log.Error("failed to get the snapshot", "in", "CalcDifficulty", "hash", parent.Hash(), "number", number, "err", err)
		return nil
	}
	env, err := c.environment(chain, parent, snap, true)
	if err != nil {
		log.Error("failed to get the environment value", "in", "CalcDifficulty", "hash", parent.Hash(), "number", number, "err", err)
		return nil
	}
	validators, err := c.getNextValidators(chain, parent, snap, true)
	if err != nil {
		log.Error("failed to get validators", "in", "CalcDifficulty", "hash", parent.Hash(), "number", number, "err", err)
		return nil
	}
	scheduler, err := c.scheduler(chain, parent, env, validators.Operators, validators.Stakes)
	if err != nil {
		log.Error("failed to get scheduler", "in", "CalcDifficulty", "hash", parent.Hash(), "number", number, "err", err)
		return nil
	}

	return scheduler.difficulty(number, c.signer, c.chainConfig.IsForkedOasysExtendDifficulty(parent.Number))
}

// SealHash returns the hash of a block without vote attestation prior to it being sealed.
// So it's not the real hash of a block, just used as unique id to distinguish task
func (c *Oasys) SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	types.EncodeSigHeaderWithoutVoteAttestation(hasher, header)
	hasher.Sum(hash[:0])
	return hash
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

// GetJustifiedNumberAndHash retrieves the number and hash of the highest justified block
// within the branch including `headers` and utilizing the latest element as the head.
func (c *Oasys) GetJustifiedNumberAndHash(chain consensus.ChainHeaderReader, headers []*types.Header) (uint64, common.Hash, error) {
	if chain == nil || len(headers) == 0 || headers[len(headers)-1] == nil {
		return 0, common.Hash{}, errors.New("illegal chain or header")
	}
	head := headers[len(headers)-1]
	if !chain.Config().IsFastFinalityEnabled(head.Number) {
		return 0, chain.GetHeaderByNumber(0).Hash(), nil
	}
	snap, err := c.snapshot(chain, head.Number.Uint64(), head.Hash(), nil)
	if err != nil {
		return 0, common.Hash{}, fmt.Errorf("unexpected error when getting snapshot in GetJustifiedNumberAndHash, blockNumber: %d, blockHash: %s, error: %v", head.Number.Uint64(), head.Hash(), err)
	}

	if snap.Attestation == nil {
		if c.chainConfig.IsFastFinalityEnabled(head.Number) {
			log.Debug("once one attestation generated, attestation of snap would not be nil forever basically")
		}
		return 0, chain.GetHeaderByNumber(0).Hash(), nil
	}
	return snap.Attestation.TargetNumber, snap.Attestation.TargetHash, nil
}

// GetFinalizedHeader returns highest finalized block header.
func (c *Oasys) GetFinalizedHeader(chain consensus.ChainHeaderReader, header *types.Header) *types.Header {
	if chain == nil || header == nil {
		return nil
	}
	if !chain.Config().IsFastFinalityEnabled(header.Number) {
		return chain.GetHeaderByNumber(0)
	}
	snap, err := c.snapshot(chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		log.Warn("Unexpected error when getting snapshot in GetFinalizedHeader",
			"error", err, "blockNumber", header.Number.Uint64(), "blockHash", header.Hash())
		return nil
	}

	if snap.Attestation == nil {
		return chain.GetHeaderByNumber(0) // keep consistent with GetJustifiedNumberAndHash
	}
	return chain.GetHeader(snap.Attestation.SourceHash, snap.Attestation.SourceNumber)
}

func (c *Oasys) getNextValidators(chain consensus.ChainHeaderReader, header *types.Header, snap *Snapshot, fromHeader bool) (validators *nextValidators, err error) {
	number := header.Number.Uint64()
	if snap.Environment.IsEpoch(number) {
		if fromHeader && c.chainConfig.IsFastFinalityEnabled(header.Number) {
			if validators, err = getValidatorsFromHeader(header); err != nil {
				log.Warn("failed to get validators from header", "in", "getNextValidators", "hash", header.Hash(), "number", number, "err", err)
			}
		}
		// If not fast finality or failed to get validators from header
		if validators == nil {
			if validators, err = getNextValidators(c.chainConfig, c.ethAPI, header.ParentHash, snap.Environment.Epoch(number), number); err != nil {
				err = fmt.Errorf("failed to get next validators, blockNumber: %d, parentHash: %s, error: %v", number, header.ParentHash, err)
				return
			}
		}
	} else {
		// Notice! The owner list is empty, Don't access owners from validators taken from the snapshot.
		validators = snap.ToNextValidators()
	}
	return
}

// Converting the validator list for the extra header field.
func (c *Oasys) getExtraHeaderValueInEpoch(number *big.Int, validators *nextValidators) []byte {
	if !c.chainConfig.IsFastFinalityEnabled(number) {
		cpy := make([]common.Address, len(validators.Operators))
		copy(cpy, validators.Operators)

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

	return assembleValidators(validators)
}

// Verify the length of the Extra header field.
func (c *Oasys) verifyExtraHeaderLengthInEpoch(number *big.Int, length int) error {
	if !c.chainConfig.IsFastFinalityEnabled(number) {
		if c.chainConfig.IsForkedOasysPublication(number) {
			if length != crypto.DigestLength {
				return errInvalidEpochHash
			}
		} else if length%common.AddressLength != 0 {
			return errInvalidCheckpointValidators
		}
		return nil
	}

	// After Fast Finality enabled, go through following checks.
	if length < envValuesLen {
		return fmt.Errorf("missing environment value in extra header, length: %d", length)
	}
	// at least one validator info in extra header
	// The exact extra header validation will be done in verifyExtraHeaderValueInEpoch during Finalize
	if length < envValuesLen+validatorNumberSize+validatorInfoBytesLen {
		return fmt.Errorf("missing validator info in extra header, length: %d", length)
	}
	return nil
}

// Verify the value of the Extra header field.
func (c *Oasys) verifyExtraHeaderValueInEpoch(header *types.Header, actual []byte, actualEnv *params.EnvironmentValue, actualValidators *nextValidators) error {
	if !c.chainConfig.IsFastFinalityEnabled(header.Number) {
		expect := c.getExtraHeaderValueInEpoch(header.Number, actualValidators)
		if bytes.Equal(actual, expect) {
			return nil
		}
		if c.chainConfig.IsForkedOasysPublication(header.Number) {
			return errMismatchingEpochHash
		}
		return errMismatchingEpochValidators
	}

	// After Fast Finality enabled, go through following checks.
	env, err := getEnvironmentFromHeader(header)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualEnv.StartBlock.Bytes(), env.StartBlock.Bytes()) {
		return fmt.Errorf("mismatching start block, expected: %v, real: %v", env.StartBlock, actualEnv.StartBlock)
	}
	if !bytes.Equal(actualEnv.StartEpoch.Bytes(), env.StartEpoch.Bytes()) {
		return fmt.Errorf("mismatching start epoch, expected: %v, real: %v", env.StartEpoch, actualEnv.StartEpoch)
	}
	if !bytes.Equal(actualEnv.BlockPeriod.Bytes(), env.BlockPeriod.Bytes()) {
		return fmt.Errorf("mismatching block period, expected: %v, real: %v", env.BlockPeriod, actualEnv.BlockPeriod)
	}
	if !bytes.Equal(actualEnv.EpochPeriod.Bytes(), env.EpochPeriod.Bytes()) {
		return fmt.Errorf("mismatching epoch period, expected: %v, real: %v", env.EpochPeriod, actualEnv.EpochPeriod)
	}
	if !bytes.Equal(actualEnv.RewardRate.Bytes(), env.RewardRate.Bytes()) {
		return fmt.Errorf("mismatching reward rate, expected: %v, real: %v", env.RewardRate, actualEnv.RewardRate)
	}
	if !bytes.Equal(actualEnv.CommissionRate.Bytes(), env.CommissionRate.Bytes()) {
		return fmt.Errorf("mismatching commission rate, expected: %v, real: %v", env.CommissionRate, actualEnv.CommissionRate)
	}
	if !bytes.Equal(actualEnv.ValidatorThreshold.Bytes(), env.ValidatorThreshold.Bytes()) {
		return fmt.Errorf("mismatching validator threshold, expected: %v, real: %v", env.ValidatorThreshold, actualEnv.ValidatorThreshold)
	}
	if !bytes.Equal(actualEnv.JailThreshold.Bytes(), env.JailThreshold.Bytes()) {
		return fmt.Errorf("mismatching jail threshold, expected: %v, real: %v", env.JailThreshold, actualEnv.JailThreshold)
	}
	if !bytes.Equal(actualEnv.JailPeriod.Bytes(), env.JailPeriod.Bytes()) {
		return fmt.Errorf("mismatching jail period, expected: %v, real: %v", env.JailPeriod, actualEnv.JailPeriod)
	}

	validators, err := getValidatorsFromHeader(header)
	if err != nil {
		return err
	}
	for i := 0; i < len(validators.Operators); i++ {
		if !bytes.Equal(actualValidators.Operators[i].Bytes(), validators.Operators[i].Bytes()) {
			return fmt.Errorf("mismatching operator, i: %d, expected: %v, real: %v", i, validators.Operators[i], actualValidators.Operators[i])
		}
		if !bytes.Equal(actualValidators.Stakes[i].Bytes(), validators.Stakes[i].Bytes()) {
			return fmt.Errorf("mismatching stake, i: %d, expected: %v, real: %v", i, validators.Stakes[i], actualValidators.Stakes[i])
		}
		if !bytes.Equal(actualValidators.VoteAddresses[i][:], validators.VoteAddresses[i][:]) {
			return fmt.Errorf("mismatching vote address, i: %d, expected: %v, real: %v", i, validators.VoteAddresses[i], actualValidators.VoteAddresses[i])
		}
	}

	return nil
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
	types.EncodeSigHeader(b, header)
	return b.Bytes()
}

func (c *Oasys) addBalanceToStakeManager(state *state.StateDB, hash common.Hash, number uint64, env *params.EnvironmentValue) error {
	if !env.IsEpoch(number) || env.Epoch(number) < 3 || env.Epoch(number) > 60 {
		return nil
	}

	var (
		rewards *big.Int
		err     error
	)
	if rewards, err = getRewards(c.ethAPI, hash); err != nil {
		return fmt.Errorf("failed to get rewards, blockNumber: %d, blockHash: %s, error: %v", number, hash, err)
	}
	if rewards.Cmp(common.Big0) == 0 {
		return nil
	}

	state.AddBalance(stakeManager.address, rewards)
	log.Info("Balance added to stake manager", "hash", hash, "amount", rewards.String())
	return nil
}

func (c *Oasys) environment(chain consensus.ChainHeaderReader, header *types.Header, snap *Snapshot, fromHeader bool) (env *params.EnvironmentValue, err error) {
	number := header.Number.Uint64()
	if number < c.config.Epoch {
		return params.InitialEnvironmentValue(c.config), nil
	}

	if snap.Environment.IsEpoch(number) {
		if fromHeader && chain.Config().IsFastFinalityEnabled(header.Number) {
			if env, err = getEnvironmentFromHeader(header); err != nil {
				log.Warn("failed to get environment value from header", "in", "environment", "hash", header.Hash(), "number", number, "err", err)
			}
		}
		// If not fast finality or failed to get environment from header
		if env == nil {
			if env, err = getNextEnvironmentValue(c.ethAPI, header.ParentHash); err != nil {
				return nil, fmt.Errorf("failed to get environment value, blockNumber: %d, parentHash: %s, error: %v", number, header.ParentHash, err)
			}
		}
	} else {
		env = snap.Environment
	}

	if env.BlockPeriod.Cmp(common.Big0) == 0 {
		return nil, errors.New("invalid block period")
	}
	if env.EpochPeriod.Cmp(common.Big0) == 0 {
		return nil, errors.New("invalid epoch period")
	}
	if env.ValidatorThreshold.Cmp(common.Big0) == 0 {
		return nil, errors.New("invalid validator threshold")
	}

	return env, nil
}

func (c *Oasys) scheduler(chain consensus.ChainHeaderReader, header *types.Header,
	env *params.EnvironmentValue, validators []common.Address, stakes []*big.Int) (*scheduler, error) {
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
		// This has nothing to do with reducing block time, but it has been fixed for possible overflow.
		seed = new(big.Int).Mod(seedHash.Big(), bigMaxInt64).Int64()
	} else {
		seed = seedHash.Big().Int64()
	}

	created := newScheduler(env, env.GetFirstBlock(number),
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

func bigTo32BytesLeftPadding(value *big.Int) []byte {
	var byteArray [32]byte
	byteSlice := value.Bytes()
	copy(byteArray[32-len(byteSlice):], byteSlice)
	return byteArray[:]
}
