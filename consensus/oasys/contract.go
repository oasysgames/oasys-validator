package oasys

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/contracts"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

var (
	systemContracts = map[common.Address]bool{
		contracts.Environment.Address:  true,
		contracts.StakeManager.Address: true,
	}
	zeroAddress = common.HexToAddress("0x0000000000000000000000000000000000000000")
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

func isToSystemContract(to common.Address) bool {
	return systemContracts[to]
}

func (c *Oasys) IsSystemTransaction(tx *types.Transaction, header *types.Header) (bool, error) {
	if tx.To() == nil {
		return false, nil
	}
	sender, err := types.Sender(c.txSigner, tx)
	if err != nil {
		return false, errors.New("unauthorized transaction")
	}
	if sender == header.Coinbase && isToSystemContract(*tx.To()) && tx.GasPrice().Cmp(big.NewInt(0)) == 0 {
		return true, nil
	}
	return false, nil
}

func (c *Oasys) IsSystemContract(to *common.Address) bool {
	if to == nil {
		return false
	}
	return isToSystemContract(*to)
}

func (c *Oasys) initializeSystemContracts(
	state *state.StateDB,
	header *types.Header,
	chain core.ChainContext,
	txs *[]*types.Transaction,
	receipts *[]*types.Receipt,
	receivedTxs *[]*types.Transaction,
	usedGas *uint64,
	mining bool,
) error {
	// initialize Environment contract
	validatorThreshold, _ := new(big.Int).SetString("10000000000000000000000000", 10)
	arg := contracts.EnvironmentValue{
		StartBlock:         new(big.Int).SetUint64(0),
		StartEpoch:         new(big.Int).SetUint64(0),
		BlockPeriod:        new(big.Int).SetUint64(c.config.Period),
		EpochPeriod:        new(big.Int).SetUint64(c.config.Epoch),
		RewardRate:         new(big.Int).SetUint64(10),
		ValidatorThreshold: validatorThreshold,
		JailThreshold:      new(big.Int).SetUint64(500),
		JailPeriod:         new(big.Int).SetUint64(2),
	}
	data, err := contracts.Environment.ABI.Pack("initialize", contracts.StakeManager.Address, arg)
	if err != nil {
		panic(err)
	}
	msg := c.getSystemMessage(header.Coinbase, contracts.Environment.Address, data, common.Big0)
	err = c.applyTransaction(msg, state, header, chain, txs, receipts, receivedTxs, usedGas, mining)
	if err != nil {
		panic(err)
	}

	// initialize StakeManager contract
	data, err = contracts.StakeManager.ABI.Pack("initialize", contracts.Environment.Address)
	if err != nil {
		panic(err)
	}
	msg = c.getSystemMessage(header.Coinbase, contracts.StakeManager.Address, data, common.Big0)
	err = c.applyTransaction(msg, state, header, chain, txs, receipts, receivedTxs, usedGas, mining)
	if err != nil {
		panic(err)
	}

	return nil
}

func (c *Oasys) getSystemMessage(from, toAddress common.Address, data []byte, value *big.Int) callmsg {
	return callmsg{
		ethereum.CallMsg{
			From:     from,
			Gas:      math.MaxUint64 / 2,
			GasPrice: big.NewInt(0),
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
	chain core.ChainContext,
	txs *[]*types.Transaction,
	receipts *[]*types.Receipt,
	receivedTxs *[]*types.Transaction,
	usedGas *uint64,
	mining bool,
) (err error) {
	if msg.From() == zeroAddress {
		return nil
	}

	nonce := state.GetNonce(msg.From())
	expectedTx := types.NewTransaction(nonce, *msg.To(), msg.Value(), msg.Gas(), msg.GasPrice(), msg.Data())
	expectedHash := c.txSigner.Hash(expectedTx)

	if msg.From() == c.signer && mining {
		expectedTx, err = c.txSignFn(accounts.Account{Address: msg.From()}, expectedTx, c.chainConfig.ChainID)
		if err != nil {
			return err
		}
	} else {
		if receivedTxs == nil || len(*receivedTxs) == 0 || (*receivedTxs)[0] == nil {
			return errors.New("supposed to get a actual transaction, but get none")
		}
		actualTx := (*receivedTxs)[0]
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
		// to next
		*receivedTxs = (*receivedTxs)[1:]
	}
	state.Prepare(expectedTx.Hash(), len(*txs))
	gasUsed, err := applyMessage(msg, state, header, c.chainConfig, chain)
	if err != nil {
		return err
	}
	*txs = append(*txs, expectedTx)
	*usedGas += gasUsed
	state.Finalise(true)

	var root []byte
	receipt := types.NewReceipt(root, false, *usedGas)
	receipt.TxHash = expectedTx.Hash()
	receipt.GasUsed = gasUsed

	// Set the receipt logs and create a bloom for filtering
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
		log.Error("apply message failed", "msg", string(ret), "err", err)
	}
	return msg.Gas() - returnGas, err
}
