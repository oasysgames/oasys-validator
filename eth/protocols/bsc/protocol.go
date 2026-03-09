package bsc

import (
	"errors"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// Constants to match up protocol versions and messages
const (
	Bsc1 = 1
<<<<<<< HEAD
=======
	Bsc2 = 2
	Bsc3 = 3 // to BAL process
>>>>>>> bf0283af9fdec4daff9512e95020fb3dd9d7d4c9
)

// ProtocolName is the official short name of the `bsc` protocol used during
// devp2p capability negotiation.
const ProtocolName = "bsc"

// ProtocolVersions are the supported versions of the `bsc` protocol (first
// is primary).
<<<<<<< HEAD
var ProtocolVersions = []uint{Bsc1}

// protocolLengths are the number of implemented message corresponding to
// different protocol versions.
var protocolLengths = map[uint]uint64{Bsc1: 2}
=======
var ProtocolVersions = []uint{Bsc1, Bsc2, Bsc3}

// protocolLengths are the number of implemented message corresponding to
// different protocol versions.
var protocolLengths = map[uint]uint64{Bsc1: 2, Bsc2: 4, Bsc3: 4}
>>>>>>> bf0283af9fdec4daff9512e95020fb3dd9d7d4c9

// maxMessageSize is the maximum cap on the size of a protocol message.
const maxMessageSize = 10 * 1024 * 1024

const (
	BscCapMsg = 0x00 // bsc capability msg used upon handshake
	VotesMsg  = 0x01
)

var defaultExtra = []byte{0x00}

var (
	errMsgTooLarge    = errors.New("message too long")
	errDecode         = errors.New("invalid message")
	errInvalidMsgCode = errors.New("invalid message code")
)

// Packet represents a p2p message in the `bsc` protocol.
type Packet interface {
	Name() string // Name returns a string corresponding to the message type.
	Kind() byte   // Kind returns the message type.
}

// BscCapPacket is the network packet for bsc capability message.
type BscCapPacket struct {
	ProtocolVersion uint
	Extra           rlp.RawValue // for extension
}

// VotesPacket is the network packet for votes record.
type VotesPacket struct {
	Votes []*types.VoteEnvelope
}

func (*BscCapPacket) Name() string { return "BscCap" }
func (*BscCapPacket) Kind() byte   { return BscCapMsg }

func (*VotesPacket) Name() string { return "Votes" }
func (*VotesPacket) Kind() byte   { return VotesMsg }
<<<<<<< HEAD
=======

type GetBlocksByRangePacket struct {
	RequestId        uint64
	StartBlockHeight uint64      // The start block height expected to be obtained from
	StartBlockHash   common.Hash // The start block hash expected to be obtained from
	Count            uint64      // Get the number of blocks from the start
}

func (*GetBlocksByRangePacket) Name() string { return "GetBlocksByRange" }
func (*GetBlocksByRangePacket) Kind() byte   { return GetBlocksByRangeMsg }

// BlockData contains types.extblock + sidecars
type BlockData struct {
	Header      *types.Header
	Txs         []*types.Transaction
	Uncles      []*types.Header
	Withdrawals []*types.Withdrawal          `rlp:"optional"`
	Sidecars    types.BlobSidecars           `rlp:"optional"`
	BAL         *types.BlockAccessListEncode `rlp:"optional"`
}

// NewBlockData creates a new BlockData object from a block
func NewBlockData(block *types.Block) *BlockData {
	return &BlockData{
		Header:      block.Header(),
		Txs:         block.Transactions(),
		Uncles:      block.Uncles(),
		Withdrawals: block.Withdrawals(),
		Sidecars:    block.Sidecars(),
		BAL:         block.BAL(),
	}
}

type BlocksByRangePacket struct {
	RequestId uint64
	Blocks    []*BlockData
}

func (*BlocksByRangePacket) Name() string { return "BlocksByRange" }
func (*BlocksByRangePacket) Kind() byte   { return BlocksByRangeMsg }
>>>>>>> bf0283af9fdec4daff9512e95020fb3dd9d7d4c9
