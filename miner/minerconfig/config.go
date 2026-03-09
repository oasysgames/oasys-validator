// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package miner implements Ethereum block creation and mining.
package minerconfig

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// Default timing configurations
var (
<<<<<<< HEAD
	defaultDelayLeftOver = 50 * time.Millisecond
	defaultRecommit      = 6 * time.Second
=======
	defaultRecommit              = 10 * time.Second
	defaultMaxWaitProposalInSecs = uint64(45)

	// Extra time for finalizing and committing blocks (excludes writing to disk).
	defaultDelayLeftOver         = 25 * time.Millisecond
	defaultBidSimulationLeftOver = 30 * time.Millisecond
	// For estimation, assume 500 Mgas/s:
	//	(100M gas / 500 Mgas/s) * 1000 ms + 10 ms buffer + defaultDelayLeftOver ≈ 235 ms.
	defaultNoInterruptLeftOver = 235 * time.Millisecond
)

// Other default MEV-related configurations
var (
	defaultMevEnabled          = false
	defaultGreedyMergeTx       = true
	defaultBuilderFeeCeil      = "0"
	defaultValidatorCommission = uint64(100)
	defaultMaxBidsPerBuilder   = uint32(2) // Simple strategy: send one bid early, another near deadline
>>>>>>> bf0283af9fdec4daff9512e95020fb3dd9d7d4c9
)

// Config is the configuration parameters of mining.
type Config struct {
<<<<<<< HEAD
	Etherbase     common.Address `toml:",omitempty"` // Public address for block mining rewards
	ExtraData     hexutil.Bytes  `toml:",omitempty"` // Block extra data set by the miner
	DelayLeftOver *time.Duration // Time reserved to finalize a block(calculate root, distribute income...)
	GasFloor      uint64         // Target gas floor for mined blocks.
	GasCeil       uint64         // Target gas ceiling for mined blocks.
	GasPrice      *big.Int       // Minimum gas price for mining a transaction
	Recommit      *time.Duration // The time interval for miner to re-create mining work.
	VoteEnable    bool           // Whether to vote when mining
=======
	Etherbase              common.Address `toml:",omitempty"` // Public address for block mining rewards
	ExtraData              hexutil.Bytes  `toml:",omitempty"` // Block extra data set by the miner
	DelayLeftOver          *time.Duration `toml:",omitempty"` // Time reserved to finalize a block(calculate root, distribute income...)
	GasFloor               uint64         // Target gas floor for mined blocks.
	GasCeil                uint64         // Target gas ceiling for mined blocks.
	GasPrice               *big.Int       // Minimum gas price for mining a transaction
	Recommit               *time.Duration `toml:",omitempty"` // The time interval for miner to re-create mining work.
	VoteEnable             bool           // Whether to vote when mining
	MaxWaitProposalInSecs  *uint64        `toml:",omitempty"` // The maximum time to wait for the proposal to be done, it's aimed to prevent validator being slashed when restarting
	DisableVoteAttestation bool           // Whether to skip assembling vote attestation
	TxGasLimit             uint64         // Maximum gas for per transaction
>>>>>>> bf0283af9fdec4daff9512e95020fb3dd9d7d4c9

	DisableVoteAttestation    bool // Whether to skip assembling vote attestation
	DisableSuspiciousTxFilter bool // Whether to disable suspicious tx filter
}

// DefaultConfig contains default settings for miner.
var DefaultConfig = Config{
<<<<<<< HEAD
	GasCeil:  60_000_000, // Same as go-ethereum(v1.16.4)
=======
	GasCeil:  100000000,
>>>>>>> bf0283af9fdec4daff9512e95020fb3dd9d7d4c9
	GasPrice: big.NewInt(params.GWei),
	// The default recommit time is chosen as two seconds since
	// consensus-layer usually will wait a half slot of time(6s)
	// for payload generation. It should be enough for Geth to
	// run 3 rounds.
	Recommit:      &defaultRecommit,
	DelayLeftOver: &defaultDelayLeftOver,
}

type BuilderConfig struct {
	Address common.Address
	URL     string
}

func ApplyDefaultMinerConfig(cfg *Config) {
	if cfg == nil {
		log.Warn("ApplyDefaultMinerConfig cfg == nil")
		return
	}

	// check [Eth.Miner]
	if cfg.DelayLeftOver == nil {
		cfg.DelayLeftOver = &defaultDelayLeftOver
		log.Info("ApplyDefaultMinerConfig", "DelayLeftOver", *cfg.DelayLeftOver)
	}
	if cfg.Recommit == nil {
		cfg.Recommit = &defaultRecommit
		log.Info("ApplyDefaultMinerConfig", "Recommit", *cfg.Recommit)
	}
}
