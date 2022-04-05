package oasys

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/contracts"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	systemContracts = map[common.Address]bool{
		contracts.Environment.Address:  true,
		contracts.StakeManager.Address: true,
	}
)

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

func getInitialEnvironment(config *params.OasysConfig) *contracts.EnvironmentValue {
	validatorThreshold, _ := new(big.Int).SetString("10000000000000000000000000", 10)
	return &contracts.EnvironmentValue{
		StartBlock:         common.Big0,
		StartEpoch:         common.Big0,
		BlockPeriod:        new(big.Int).SetUint64(config.Period),
		EpochPeriod:        new(big.Int).SetUint64(config.Epoch),
		RewardRate:         big.NewInt(10),
		ValidatorThreshold: validatorThreshold,
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
	// initialize Environment contract
	data, err := contracts.Environment.ABI.Pack("initialize", contracts.StakeManager.Address, getInitialEnvironment(c.config))
	if err != nil {
		return err
	}
	msg := c.getSystemMessage(header.Coinbase, contracts.Environment.Address, data, common.Big0)
	err = c.applyTransaction(msg, state, header, cx, txs, receipts, systemTxs, usedGas, mining)
	if err != nil {
		return err
	}

	// initialize StakeManager contract
	data, err = contracts.StakeManager.ABI.Pack("initialize", contracts.Environment.Address)
	if err != nil {
		return err
	}
	msg = c.getSystemMessage(header.Coinbase, contracts.StakeManager.Address, data, common.Big0)
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
	data, err := contracts.StakeManager.ABI.Pack("updateValidatorBlocks", validators, blocks)
	if err != nil {
		return err
	}
	msg := c.getSystemMessage(header.Coinbase, contracts.StakeManager.Address, data, common.Big0)
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
	data, err := contracts.StakeManager.ABI.Pack("updateValidators")
	if err != nil {
		return err
	}
	msg := c.getSystemMessage(header.Coinbase, contracts.StakeManager.Address, data, common.Big0)
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
	data, err := contracts.StakeManager.ABI.Pack("slash", validator)
	if err != nil {
		return err
	}
	msg := c.getSystemMessage(header.Coinbase, contracts.StakeManager.Address, data, common.Big0)
	return c.applyTransaction(msg, state, header, cx, txs, receipts, systemTxs, usedGas, mining)
}

func getNextValidators(ethAPI *ethapi.PublicBlockChainAPI, hash common.Hash) (*contracts.GetNextValidatorsResult, error) {
	method := "getCurrentValidators"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data, err := contracts.StakeManager.ABI.Pack(method)
	if err != nil {
		return nil, err
	}

	hexData := (hexutil.Bytes)(data)
	rbytes, err := ethAPI.Call(
		ctx,
		ethapi.TransactionArgs{
			To:   &contracts.StakeManager.Address,
			Data: &hexData,
		},
		rpc.BlockNumberOrHashWithHash(hash, false),
		nil)
	if err != nil {
		return nil, err
	}

	var result contracts.GetNextValidatorsResult
	if err := contracts.StakeManager.ABI.UnpackIntoInterface(&result, method, rbytes); err != nil {
		return nil, err
	}

	return &result, nil
}

func getAllAmounts(ethAPI *ethapi.PublicBlockChainAPI, hash common.Hash) (*contracts.GetAllAmountsResult, error) {
	method := "getAllAmounts"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data, err := contracts.StakeManager.ABI.Pack(method)
	if err != nil {
		return nil, err
	}

	hexData := (hexutil.Bytes)(data)
	result, err := ethAPI.Call(
		ctx,
		ethapi.TransactionArgs{
			To:   &contracts.StakeManager.Address,
			Data: &hexData,
		},
		rpc.BlockNumberOrHashWithHash(hash, false),
		nil)
	if err != nil {
		return nil, err
	}

	var amounts contracts.GetAllAmountsResult

	if err := contracts.StakeManager.ABI.UnpackIntoInterface(&amounts, method, result); err != nil {
		return nil, err
	}

	return &amounts, nil
}

func getNextEnvironmentValue(ethAPI *ethapi.PublicBlockChainAPI, hash common.Hash) (*contracts.EnvironmentValue, error) {
	method := "nextValue"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	data, err := contracts.Environment.ABI.Pack(method)
	if err != nil {
		return nil, err
	}

	hexData := (hexutil.Bytes)(data)
	rbytes, err := ethAPI.Call(
		ctx,
		ethapi.TransactionArgs{
			To:   &contracts.Environment.Address,
			Data: &hexData,
		},
		rpc.BlockNumberOrHashWithHash(hash, false),
		nil)
	if err != nil {
		return nil, err
	}

	var result struct{ Result contracts.EnvironmentValue }
	if err := contracts.Environment.ABI.UnpackIntoInterface(&result, method, rbytes); err != nil {
		return nil, err
	}

	return &result.Result, nil
}

func (c *Oasys) getSystemMessage(from, toAddress common.Address, data []byte, value *big.Int) callmsg {
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
