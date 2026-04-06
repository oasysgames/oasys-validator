package log

import "github.com/ethereum/go-ethereum/common"

// To bypass the AMD plugin build error, we temporarily define the Log struct locally instead of importing the `core/types` package.
// Error:
//
//	go: downloading github.com/ethereum/c-kzg-4844/v2 v2.1.0
//	# github.com/consensys/gnark-crypto/ecc/bls12-381/internal/fptower
//	asm: e2_amd64.s:68: when dynamic linking, R15 is clobbered by a global variable access and is used here: 00069 (/home/azureuser/go/pkg/mod/github.com/consensys/gnark-crypto@v0.18.0/ecc/bls12-381/internal/fptower/e2_amd64.s:68)	CMOVQCS	R15, R9
//	asm: e2_amd64.s:117: when dynamic linking, R15 is clobbered by a global variable access and is used here: 00146 (/home/azureuser/go/pkg/mod/github.com/consensys/gnark-crypto@v0.18.0/ecc/bls12-381/internal/fptower/e2_amd64.s:117)	CMOVQCS	R15, R9
//	asm: e2_amd64.s:355: when dynamic linking, R15 is clobbered by a global variable access and is used here: 00388 (/home/azureuser/go/pkg/mod/github.com/consensys/gnark-crypto@v0.18.0/ecc/bls12-381/internal/fptower/e2_amd64.s:355)	MOVQ	R8, 48(R15)
//	asm: e2_amd64.s:575: when dynamic linking, R15 is clobbered by a global variable access and is used here: 01074 (/home/azureuser/go/pkg/mod/github.com/consensys/gnark-crypto@v0.18.0/ecc/bls12-381/internal/fptower/e2_amd64.s:575)	CMOVQCS	R15, R9
//	asm: e2_amd64.s:751: when dynamic linking, R15 is clobbered by a global variable access and is used here: 02059 (/home/azureuser/go/pkg/mod/github.com/consensys/gnark-crypto@v0.18.0/ecc/bls12-381/internal/fptower/e2_amd64.s:751)	CMOVQCS	R15, R9
//	asm: assembly failed
//
// Copied from `github.com/ethereum/go-ethereum/core/types/log.go`
type Log struct {
	// Consensus fields:
	// address of the contract that generated the event
	Address common.Address `json:"address" gencodec:"required"`
	// list of topics provided by the contract.
	Topics []common.Hash `json:"topics" gencodec:"required"`
	// supplied by the contract, usually ABI-encoded
	Data []byte `json:"data" gencodec:"required"`

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber uint64 `json:"blockNumber" rlp:"-"`
	// hash of the transaction
	TxHash common.Hash `json:"transactionHash" gencodec:"required" rlp:"-"`
	// index of the transaction in the block
	TxIndex uint `json:"transactionIndex" rlp:"-"`
	// hash of the block in which the transaction was included
	BlockHash common.Hash `json:"blockHash" rlp:"-"`
	// timestamp of the block in which the transaction was included
	BlockTimestamp uint64 `json:"blockTimestamp" rlp:"-"`
	// index of the log in the block
	Index uint `json:"logIndex" rlp:"-"`

	// The Removed field is true if this log was reverted due to a chain reorganisation.
	// You must pay attention to this field if you receive logs through a filter query.
	Removed bool `json:"removed" rlp:"-"`
}
