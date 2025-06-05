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

var (
	defaultDelayLeftOver         = 50 * time.Millisecond
	defaultRecommit              = 6 * time.Second
	defaultMaxWaitProposalInSecs = uint64(45)
	// default configurations for MEV
	defaultGreedyMergeTx         bool   = true
	defaultValidatorCommission   uint64 = 100
	defaultBidSimulationLeftOver        = 50 * time.Millisecond
	defaultNoInterruptLeftOver          = 250 * time.Millisecond
	defaultMaxBidsPerBuilder     uint32 = 2
)

// Config is the configuration parameters of mining.
type Config struct {
	Etherbase     common.Address `toml:",omitempty"` // Public address for block mining rewards
	ExtraData     hexutil.Bytes  `toml:",omitempty"` // Block extra data set by the miner
	DelayLeftOver *time.Duration // Time reserved to finalize a block(calculate root, distribute income...)
	GasFloor      uint64         // Target gas floor for mined blocks.
	GasCeil       uint64         // Target gas ceiling for mined blocks.
	GasPrice      *big.Int       // Minimum gas price for mining a transaction
	Recommit      *time.Duration // The time interval for miner to re-create mining work.
	VoteEnable    bool           // Whether to vote when mining

	DisableVoteAttestation bool // Whether to skip assembling vote attestation
}

// DefaultConfig contains default settings for miner.
var DefaultConfig = Config{
	GasCeil:  30000000,
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
