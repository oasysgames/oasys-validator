package oasys

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/contracts"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	addressPrefix = contracts.AddressPrefix["genesis"].Hash().Big()

	// Oasys system contracts
	environment = &systemContract{
		address: common.BigToAddress(new(big.Int).Add(addressPrefix, big.NewInt(4096*1))),
		abis:    environmentAbi,
	}
	stakeManager = &systemContract{
		address: common.BigToAddress(new(big.Int).Add(addressPrefix, big.NewInt(4096*2))),
		abis:    stakeManagerAbi,
	}
	systemContracts = map[common.Address]bool{environment.address: true, stakeManager.address: true}
)

func init() {
	// Parse the system contract ABI
	contracts := []*systemContract{environment, stakeManager}
	for _, contract := range contracts {
		ABI, err := abi.JSON(strings.NewReader(contract.abis))
		if err != nil {
			panic(err)
		}
		contract.abi = &ABI
	}
}

// systemContract
type systemContract struct {
	address common.Address
	abi     *abi.ABI
	abis    string
}

// chainContext
type chainContext struct {
	Chain consensus.ChainHeaderReader
	oasys consensus.Engine
}

func (c chainContext) Engine() consensus.Engine {
	return c.oasys
}

func (c chainContext) GetHeader(hash common.Hash, number uint64) *types.Header {
	return c.Chain.GetHeader(hash, number)
}

// environmentValue
type environmentValue struct {
	// Block and epoch to which this setting applies
	StartBlock *big.Int
	StartEpoch *big.Int
	// Block generation interval(by seconds)
	BlockPeriod *big.Int
	// Number of blocks in epoch
	EpochPeriod *big.Int
	// Annual rate of staking reward
	RewardRate *big.Int
	// Amount of tokens required to become a validator
	ValidatorThreshold *big.Int
	// Number of not sealed to jailing the validator
	JailThreshold *big.Int
	// Number of epochs to jailing the validator
	JailPeriod *big.Int
}

func (p *environmentValue) IsEpoch(number uint64) bool {
	return number%p.EpochPeriod.Uint64() == 0
}

func (p *environmentValue) Epoch(number uint64) uint64 {
	return p.StartEpoch.Uint64() + (number-p.StartBlock.Uint64())/p.EpochPeriod.Uint64()
}

func (p *environmentValue) Copy() *environmentValue {
	return &environmentValue{
		StartBlock:         new(big.Int).Set(p.StartBlock),
		StartEpoch:         new(big.Int).Set(p.StartEpoch),
		BlockPeriod:        new(big.Int).Set(p.BlockPeriod),
		EpochPeriod:        new(big.Int).Set(p.EpochPeriod),
		RewardRate:         new(big.Int).Set(p.RewardRate),
		ValidatorThreshold: new(big.Int).Set(p.ValidatorThreshold),
		JailThreshold:      new(big.Int).Set(p.JailThreshold),
		JailPeriod:         new(big.Int).Set(p.JailPeriod),
	}
}

// getNextValidatorsResult
type getNextValidatorsResult struct {
	Owners    []common.Address
	Operators []common.Address
	Stakes    []*big.Int
}

func (p *getNextValidatorsResult) Copy() *getNextValidatorsResult {
	cpy := getNextValidatorsResult{
		Owners:    make([]common.Address, len(p.Owners)),
		Operators: make([]common.Address, len(p.Operators)),
		Stakes:    make([]*big.Int, len(p.Stakes)),
	}
	copy(cpy.Owners, p.Owners)
	copy(cpy.Operators, p.Operators)
	for i, v := range p.Stakes {
		cpy.Stakes[i] = new(big.Int).Set(v)
	}
	return &cpy
}

func (p *getNextValidatorsResult) Exists(validator common.Address) bool {
	for _, operator := range p.Operators {
		if validator == operator {
			return true
		}
	}
	return false
}

func getInitialEnvironment(config *params.OasysConfig) *environmentValue {
	return &environmentValue{
		StartBlock:         common.Big0,
		StartEpoch:         common.Big0,
		BlockPeriod:        big.NewInt(int64(config.Period)),
		EpochPeriod:        big.NewInt(int64(config.Epoch)),
		RewardRate:         big.NewInt(10),
		ValidatorThreshold: new(big.Int).Mul(big.NewInt(10000000), big.NewInt(1e18)),
		JailThreshold:      big.NewInt(500),
		JailPeriod:         big.NewInt(2),
	}
}

// callmsg
type callmsg struct {
	ethereum.CallMsg
}

func (m callmsg) From() common.Address { return m.CallMsg.From }
func (m callmsg) Nonce() uint64        { return 0 }
func (m callmsg) CheckNonce() bool     { return false }
func (m callmsg) To() *common.Address  { return m.CallMsg.To }
func (m callmsg) GasPrice() *big.Int   { return m.CallMsg.GasPrice }
func (m callmsg) Gas() uint64          { return m.CallMsg.Gas }
func (m callmsg) Value() *big.Int      { return m.CallMsg.Value }
func (m callmsg) Data() []byte         { return m.CallMsg.Data }

func (c *Oasys) IsSystemTransaction(tx *types.Transaction, header *types.Header) (bool, error) {
	if tx.To() == nil {
		return false, nil
	}
	sender, err := types.Sender(c.txSigner, tx)
	if err != nil {
		return false, errors.New("unauthorized transaction")
	}
	if sender == header.Coinbase && systemContracts[*tx.To()] && tx.GasPrice().Cmp(common.Big0) == 0 {
		return true, nil
	}
	return false, nil
}

// update functions
func (c *Oasys) initializeSystemContracts(
	state *state.StateDB,
	header *types.Header,
	cx core.ChainContext,
	txs *[]*types.Transaction,
	receipts *[]*types.Receipt,
	systemTxs *[]*types.Transaction,
	usedGas *uint64,
	mining bool,
) error {
	// Initialize Environment contract
	data, err := environment.abi.Pack("initialize", getInitialEnvironment(c.config))
	if err != nil {
		return err
	}
	msg := getMessage(header.Coinbase, environment.address, data, common.Big0)
	err = c.applyTransaction(msg, state, header, cx, txs, receipts, systemTxs, usedGas, mining)
	if err != nil {
		return err
	}

	// Initialize StakeManager contract
	data, err = stakeManager.abi.Pack("initialize", environment.address)
	if err != nil {
		return err
	}
	msg = getMessage(header.Coinbase, stakeManager.address, data, common.Big0)
	err = c.applyTransaction(msg, state, header, cx, txs, receipts, systemTxs, usedGas, mining)
	if err != nil {
		return err
	}

	return nil
}

func (c *Oasys) updateValidatorBlocks(
	schedule map[uint64]common.Address,
	state *state.StateDB,
	header *types.Header,
	cx core.ChainContext,
	txs *[]*types.Transaction,
	receipts *[]*types.Receipt,
	systemTxs *[]*types.Transaction,
	usedGas *uint64,
	mining bool,
) error {
	validators, blocks := getValidatorBlocks(schedule)
	data, err := stakeManager.abi.Pack("updateValidatorBlocks", validators, blocks)
	if err != nil {
		return err
	}
	msg := getMessage(header.Coinbase, stakeManager.address, data, common.Big0)
	err = c.applyTransaction(msg, state, header, cx, txs, receipts, systemTxs, usedGas, mining)
	if err != nil {
		return err
	}

	return nil
}

func (c *Oasys) updateValidators(
	state *state.StateDB,
	header *types.Header,
	cx core.ChainContext,
	txs *[]*types.Transaction,
	receipts *[]*types.Receipt,
	systemTxs *[]*types.Transaction,
	usedGas *uint64,
	mining bool,
) error {
	data, err := stakeManager.abi.Pack("updateValidators")
	if err != nil {
		return err
	}
	msg := getMessage(header.Coinbase, stakeManager.address, data, common.Big0)
	return c.applyTransaction(msg, state, header, cx, txs, receipts, systemTxs, usedGas, mining)
}

func (c *Oasys) slash(
	validator common.Address,
	state *state.StateDB,
	header *types.Header,
	cx core.ChainContext,
	txs *[]*types.Transaction,
	receipts *[]*types.Receipt,
	systemTxs *[]*types.Transaction,
	usedGas *uint64,
	mining bool,
) error {
	data, err := stakeManager.abi.Pack("slash", validator)
	if err != nil {
		return err
	}
	msg := getMessage(header.Coinbase, stakeManager.address, data, common.Big0)
	return c.applyTransaction(msg, state, header, cx, txs, receipts, systemTxs, usedGas, mining)
}

// view functions
func getNextValidators(ethAPI *ethapi.PublicBlockChainAPI, hash common.Hash) (*getNextValidatorsResult, error) {
	method := "getCurrentValidators"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data, err := stakeManager.abi.Pack(method)
	if err != nil {
		return nil, err
	}

	hexData := (hexutil.Bytes)(data)
	rbytes, err := ethAPI.Call(
		ctx,
		ethapi.TransactionArgs{
			To:   &stakeManager.address,
			Data: &hexData,
		},
		rpc.BlockNumberOrHashWithHash(hash, false),
		nil)
	if err != nil {
		return nil, err
	}

	var result getNextValidatorsResult
	if err := stakeManager.abi.UnpackIntoInterface(&result, method, rbytes); err != nil {
		return nil, err
	}

	return &result, nil
}

func getRewards(ethAPI *ethapi.PublicBlockChainAPI, hash common.Hash) (*big.Int, error) {
	method := "getTotalRewards"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data, err := stakeManager.abi.Pack(method, common.Big1)
	if err != nil {
		return nil, err
	}

	hexData := (hexutil.Bytes)(data)
	rbytes, err := ethAPI.Call(
		ctx,
		ethapi.TransactionArgs{
			To:   &stakeManager.address,
			Data: &hexData,
		},
		rpc.BlockNumberOrHashWithHash(hash, false),
		nil)
	if err != nil {
		return nil, err
	}

	var result *big.Int
	if err := stakeManager.abi.UnpackIntoInterface(&result, method, rbytes); err != nil {
		return nil, err
	}

	return result, nil
}

func getNextEnvironmentValue(ethAPI *ethapi.PublicBlockChainAPI, hash common.Hash) (*environmentValue, error) {
	method := "nextValue"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data, err := environment.abi.Pack(method)
	if err != nil {
		return nil, err
	}

	hexData := (hexutil.Bytes)(data)
	rbytes, err := ethAPI.Call(
		ctx,
		ethapi.TransactionArgs{
			To:   &environment.address,
			Data: &hexData,
		},
		rpc.BlockNumberOrHashWithHash(hash, false),
		nil)
	if err != nil {
		return nil, err
	}

	var result struct{ Result environmentValue }
	if err := environment.abi.UnpackIntoInterface(&result, method, rbytes); err != nil {
		return nil, err
	}

	return &result.Result, nil
}

func (c *Oasys) applyTransaction(
	msg callmsg,
	state *state.StateDB,
	header *types.Header,
	cx core.ChainContext,
	txs *[]*types.Transaction,
	receipts *[]*types.Receipt,
	systemTxs *[]*types.Transaction,
	usedGas *uint64,
	mining bool,
) (err error) {
	nonce := state.GetNonce(msg.From())
	expectedTx := types.NewTransaction(nonce, *msg.To(), msg.Value(), msg.Gas(), msg.GasPrice(), msg.Data())
	expectedHash := c.txSigner.Hash(expectedTx)

	if msg.From() == c.signer && mining {
		expectedTx, err = c.txSignFn(accounts.Account{Address: msg.From()}, expectedTx, c.chainConfig.ChainID)
		if err != nil {
			return err
		}
	} else {
		if systemTxs == nil || len(*systemTxs) == 0 || (*systemTxs)[0] == nil {
			return errors.New("supposed to get a actual transaction, but get none")
		}
		actualTx := (*systemTxs)[0]
		if !bytes.Equal(c.txSigner.Hash(actualTx).Bytes(), expectedHash.Bytes()) {
			return fmt.Errorf("expected tx hash %v, get %v, nonce %d, to %s, value %s, gas %d, gasPrice %s, data %s", expectedHash.String(), actualTx.Hash().String(),
				expectedTx.Nonce(),
				expectedTx.To().String(),
				expectedTx.Value().String(),
				expectedTx.Gas(),
				expectedTx.GasPrice().String(),
				hex.EncodeToString(expectedTx.Data()),
			)
		}
		expectedTx = actualTx
		*systemTxs = (*systemTxs)[1:]
	}
	state.Prepare(expectedTx.Hash(), len(*txs))
	gasUsed, err := applyMessage(msg, state, header, c.chainConfig, cx)
	if err != nil {
		return err
	}
	*txs = append(*txs, expectedTx)
	var root []byte
	if c.chainConfig.IsByzantium(header.Number) {
		state.Finalise(true)
	} else {
		root = state.IntermediateRoot(c.chainConfig.IsEIP158(header.Number)).Bytes()
	}
	*usedGas += gasUsed
	receipt := types.NewReceipt(root, false, *usedGas)
	receipt.TxHash = expectedTx.Hash()
	receipt.GasUsed = gasUsed

	receipt.Logs = state.GetLogs(expectedTx.Hash(), common.Hash{})
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	receipt.BlockHash = common.Hash{}
	receipt.BlockNumber = header.Number
	receipt.TransactionIndex = uint(state.TxIndex())
	*receipts = append(*receipts, receipt)
	state.SetNonce(msg.From(), nonce+1)
	return nil
}

func getMessage(from, toAddress common.Address, data []byte, value *big.Int) callmsg {
	return callmsg{
		ethereum.CallMsg{
			From:     from,
			Gas:      math.MaxUint64 / 2,
			GasPrice: common.Big0,
			Value:    value,
			To:       &toAddress,
			Data:     data,
		},
	}
}

func applyMessage(
	msg callmsg,
	state *state.StateDB,
	header *types.Header,
	chainConfig *params.ChainConfig,
	chain core.ChainContext,
) (uint64, error) {
	context := core.NewEVMBlockContext(header, chain, nil)
	vmenv := vm.NewEVM(context, vm.TxContext{Origin: msg.From(), GasPrice: big.NewInt(0)}, state, chainConfig, vm.Config{})
	ret, returnGas, err := vmenv.Call(
		vm.AccountRef(msg.From()),
		*msg.To(),
		msg.Data(),
		msg.Gas(),
		msg.Value(),
	)
	if err != nil {
		log.Error("failed apply message", "msg", string(ret), "err", err)
	}
	return msg.Gas() - returnGas, err
}
