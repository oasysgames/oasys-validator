package oasys

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

var deploymentSets = map[common.Hash]map[uint64]deploymentSet{
	params.OasysMainnetGenesisHash: {
		1:       deploymentSet{deployments0},
		235000:  deploymentSet{deployments1},
		309600:  deploymentSet{deployments2, deployments3, deployments4},
		419000:  deploymentSet{deployments5},
		557100:  deploymentSet{deployments6},
		971800:  deploymentSet{deployments7},
		1529980: deploymentSet{deployments9},
		1892000: deploymentSet{deployments10},
		4089588: deploymentSet{deployments11},
	},
	params.OasysTestnetGenesisHash: {
		1:       deploymentSet{deployments0},
		189400:  deploymentSet{deployments2},
		200800:  deploymentSet{deployments1},
		269700:  deploymentSet{deployments3},
		293000:  deploymentSet{deployments4},
		385000:  deploymentSet{deployments5},
		546400:  deploymentSet{deployments6},
		955400:  deploymentSet{deployments7, deployments8},
		1519840: deploymentSet{deployments9},
		1880660: deploymentSet{deployments10},
		4017600: deploymentSet{deployments11},
	},
	defaultGenesisHash: {
		2: deploymentSet{
			deployments0,
			deployments1,
			deployments2,
			deployments3,
			deployments4,
			deployments5,
			deployments6,
			deployments7,
			deployments8,
			deployments9,
			deployments10,
			deployments11,
		},
	},
}

func mustDecodeCode(code string) []byte {
	hexcode, err := hex.DecodeString(strings.TrimPrefix(code, hexPrefix))
	if err != nil {
		panic(fmt.Sprintf("failed to decode the contract code: %s\n", err.Error()))
	}
	return hexcode
}

type deploymentSet [][]*deployment

// Contract deployment definition.
type deployment struct {
	contract *contract
	code     []byte
	storage  storage
}

// Deploy the contract.
func (d *deployment) deploy(cfg *params.ChainConfig, state StateDB, block uint64) {
	d.deployCode(state)
	d.deployStorage(cfg, state)
	log.Info("Deploy contract", "block", block,
		"name", d.contract.name, "address", d.contract.address)
}

func (d *deployment) deployCode(state StateDB) {
	if d.code != nil {
		state.SetCode(common.HexToAddress(d.contract.address), d.code)
	}
}

func (d *deployment) deployStorage(cfg *params.ChainConfig, state StateDB) {
	storage, err := d.storage.build(cfg)
	if err != nil {
		panic(fmt.Errorf("failed to build %s contract storage map: %s",
			d.contract.name, err.Error()))
	}

	address := common.HexToAddress(d.contract.address)
	for key, val := range storage {
		state.SetState(address, key, val)
	}
}
