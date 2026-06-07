package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/txfilter/plugintransfer/config"
)

const (
	// From genesis accounts in oasys-private-l1
	// Reference: https://github.com/oasysgames/oasys-private-l1/blob/main/l1/validator/genesis.json
	// Faucet: 0x708c87fBbec51DE4EDa4E18A872222648316BCB5
	priv1 = "0x9ae97161da58263758cc57459bc39252bab369893324d401cb5abe2f6e2e6ce4"
	// L2 Admin: 0xccf3e6b439D0B0546fc2ac48afb3f2Cac0c84d26
	priv2 = "0x46973b86922973b52ca2936f20250e0b6d409563998a04f746ad21bf53b2cd5b"

	// URL of blockchain RPC
	blockchainRPCURL = "http://localhost:8545"

	// URL of plugin config
	pluginConfigURL = "http://localhost:3030/suspicious_txfilter_config.json"

	// If a tx has no receipt after this duration, consider it blocked by the plugin.
	blockedTxTimeout = 10 * time.Second
)

var (
	whitelistPrivs = []string{
		// Hardhat Account #0: 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
		"0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		// Hardhat Account #1: 0x70997970C51812dc3A010C7d01b50e0d17dc79C8
		"0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
	}

	// ERC20 Transfer(address,uint256) selector
	erc20TransferSelector = []byte{0xa9, 0x05, 0x9c, 0xbb}

	// Destination address for transfers Native and ERC20
	destinationAddress = common.HexToAddress("0x8626f6940E2eb28930eFb4CeF49B2d1F2C9C1199")

	client            *ethclient.Client
	cfg               config.PluginConfig
	privKyes          []*ecdsaKey
	whitelistKeys     []*ecdsaKey
	chainID           *big.Int
	targetERC20       common.Address
	targetERC20Config config.TargetERC20Config
	nextTrasferKind   = 0
)

func TestPluginTransferE2E(t *testing.T) {
	t.Logf("Starting PluginTransferE2E test...")

	ctx := context.Background()
	var err error

	// Access both blockchain and plugin config URL; skip if either fails
	if client, err = ethclient.Dial(blockchainRPCURL); err != nil {
		t.Skipf("blockchain RPC not available (%s): %v", blockchainRPCURL, err)
	}
	defer client.Close()

	if _, err := client.ChainID(ctx); err != nil {
		t.Skipf("blockchain RPC not ready: %v", err)
	}

	resp, err := http.Get(pluginConfigURL)
	if err != nil {
		t.Skipf("plugin config URL not available (%s): %v", pluginConfigURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("plugin config URL returned %d", resp.StatusCode)
	}

	// Load plugin config from endpoint
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode plugin config: %v", err)
	}

	// Assert whitelist: whitelistPrivs addresses are in whitelist; priv1/priv2 are not
	whitelistAddrs := mustAddressesFromPrivs(t, whitelistPrivs)
	for i, addr := range whitelistAddrs {
		if !cfg.Whitelists[addr] {
			t.Fatalf("whitelistPrivs[%d] address %s must be in config whitelist", i, addr.Hex())
		}
	}
	priv1Addr := mustAddressFromPriv(t, priv1)
	priv2Addr := mustAddressFromPriv(t, priv2)
	if cfg.Whitelists[priv1Addr] {
		t.Fatalf("priv1 address %s must not be in config whitelist", priv1Addr.Hex())
	}
	if cfg.Whitelists[priv2Addr] {
		t.Fatalf("priv2 address %s must not be in config whitelist", priv2Addr.Hex())
	}

	// Set common variables
	if chainID, err = client.ChainID(ctx); err != nil {
		t.Fatalf("get chain ID: %v", err)
	}
	privKyes = make([]*ecdsaKey, 2)
	privKyes[0] = mustPrivKey(t, priv1)
	privKyes[1] = mustPrivKey(t, priv2)
	whitelistKeys = make([]*ecdsaKey, len(whitelistPrivs))
	for i, priv := range whitelistPrivs {
		whitelistKeys[i] = mustPrivKey(t, priv)
	}
	targetERC20 = cfg.TargetERC20s[0].Address
	targetERC20Config = cfg.TargetERC20s[0]

	// Count threshold scenario
	t.Logf("Starting CountThreshold scenario...")
	t.Run("CountThreshold", func(t *testing.T) {
		runCountThresholdScenario(t, ctx)
	})

	// Wait measurement window to reset count threshold
	t.Logf("Waiting for measurement window to reset count threshold..., window: %s", cfg.MeasurementWindow.String())
	time.Sleep(cfg.MeasurementWindow)

	// Amount threshold scenario
	t.Logf("Starting AmountThreshold scenario...")
	t.Run("AmountThreshold", func(t *testing.T) {
		runAmountThresholdScenario(t, ctx)
	})
}

func runCountThresholdScenario(t *testing.T, ctx context.Context) {
	var (
		thresholdCount          = cfg.Threshold.BlockCountThreshold
		nativeValue, erc20Value = computeYenValue(cfg.NativeToken, targetERC20Config, 10_000)
		privKey                 = privKyes[0]
	)

	// 1st: send (block_count_threshold - 1) txs from non-whitelist
	for i := uint(0); i < thresholdCount; i++ {
		receipt := sendTransferAndWaitReceipt(t, ctx, privKey, &nativeValue, &erc20Value)
		if receipt == nil {
			t.Fatalf("count threshold: tx %d from non-whitelist (under threshold) expected mined within %v", i+1, blockedTxTimeout)
		}
	}

	// 2nd: send one more from priv1 — should exceed count threshold and be blocked (no receipt in 10s)
	receipt := sendTransferAndWaitReceipt(t, ctx, privKey, &nativeValue, &erc20Value)
	if receipt != nil {
		t.Fatalf("count threshold: tx from non-whitelist that exceeds threshold should be blocked (no receipt within %v), but got receipt block %d", blockedTxTimeout, receipt.BlockNumber.Uint64())
	}

	// Whitelist can still send
	receipt = sendTransferAndWaitReceipt(t, ctx, whitelistKeys[0], &nativeValue, &erc20Value)
	if receipt == nil {
		t.Fatalf("count threshold: tx from whitelist should be mined within %v", blockedTxTimeout)
	}
}

func runAmountThresholdScenario(t *testing.T, ctx context.Context) {
	var (
		thresholdCount  = cfg.Threshold.BlockCountThreshold
		loopCount       = thresholdCount - 2
		thresholdAmount = cfg.Threshold.BlockAmountThreshold
		// compute X yen that is 1/10th of block_amount_threshold
		nativeValue, erc20Value = computeYenValue(cfg.NativeToken, targetERC20Config, thresholdAmount/uint64(loopCount))
		privKey                 = privKyes[1]
	)

	// 1st: send small amount from priv1 — should be mined (random: native / ERC20 / both)
	for i := uint(0); i < loopCount; i++ {
		receipt := sendTransferAndWaitReceipt(t, ctx, privKey, &nativeValue, &erc20Value)
		if receipt == nil {
			t.Fatalf("amount threshold: small tx from priv1 expected mined within %v", blockedTxTimeout)
		}
	}

	// 2nd: send amount that exceeds block_amount_threshold — should be blocked (random choice; over-threshold amounts)
	receipt := sendTransferAndWaitReceipt(t, ctx, privKey, &nativeValue, &erc20Value)
	if receipt != nil {
		t.Fatalf("amount threshold: tx from priv1 that exceeds amount threshold should be blocked (no receipt within %v), but got receipt block %d", blockedTxTimeout, receipt.BlockNumber.Uint64())
	}

	// Whitelist can still send
	receipt = sendTransferAndWaitReceipt(t, ctx, whitelistKeys[0], &nativeValue, &erc20Value)
	if receipt == nil {
		t.Fatalf("amount threshold: tx from whitelist should be mined within %v", blockedTxTimeout)
	}
}

// transferKind is the random choice for what to send in sendRandomTransferAndWaitReceipt.
type transferKind int

const (
	kindNative transferKind = iota
	kindERC20
)

func sendTransferAndWaitReceipt(t *testing.T, ctx context.Context, key *ecdsaKey, valueNative, valueToken *big.Int) *types.Receipt {
	t.Helper()
	kind := transferKind(nextTrasferKind)
	nextTrasferKind = (nextTrasferKind + 1) % 2

	switch kind {
	case kindNative:
		return sendNativeTransferAndWaitReceipt(t, ctx, key, valueNative)
	case kindERC20:
		return sendERC20TransferAndWaitReceipt(t, ctx, key, targetERC20, valueToken)
	}
	return nil
}

// sendNativeTransferAndWaitReceipt sends a simple ETH transfer and polls for receipt.
func sendNativeTransferAndWaitReceipt(t *testing.T, ctx context.Context, key *ecdsaKey, value *big.Int) *types.Receipt {
	t.Helper()
	nonce, err := client.PendingNonceAt(ctx, key.addr)
	if err != nil {
		t.Fatalf("PendingNonceAt: %v", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatalf("SuggestGasPrice: %v", err)
	}
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &destinationAddress,
		Value:    value,
		Gas:      params.TxGas,
		GasPrice: gasPrice,
	})
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), key.key)
	if err != nil {
		t.Fatalf("SignTx: %v", err)
	}
	t.Logf("sending native transfer tx: %s, value: %s", signed.Hash().Hex(), value.String())
	return sendTxAndWaitReceipt(t, ctx, signed)
}

func sendERC20TransferAndWaitReceipt(t *testing.T, ctx context.Context, key *ecdsaKey, tokenContract common.Address, amountToken *big.Int) *types.Receipt {
	t.Helper()
	nonce, err := client.PendingNonceAt(ctx, key.addr)
	if err != nil {
		t.Fatalf("PendingNonceAt: %v", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatalf("SuggestGasPrice: %v", err)
	}
	data := encodeERC20Transfer(destinationAddress, amountToken)
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &tokenContract,
		Value:    big.NewInt(0),
		Gas:      100000,
		GasPrice: gasPrice,
		Data:     data,
	})
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), key.key)
	if err != nil {
		t.Fatalf("SignTx: %v", err)
	}
	t.Logf("sending ERC20 transfer tx: %s, amountToken: %s", signed.Hash().Hex(), amountToken.String())
	return sendTxAndWaitReceipt(t, ctx, signed)
}

// encodeERC20Transfer returns calldata for Transfer(address to, uint256 value).
func encodeERC20Transfer(to common.Address, amount *big.Int) []byte {
	// selector (4) + to (32) + value (32)
	b := make([]byte, 4+32+32)
	copy(b, erc20TransferSelector)
	copy(b[4+12:], to.Bytes()) // left-pad address to 32 bytes
	amount.FillBytes(b[4+32:]) // big-endian uint256
	return b
}

// sendTxAndWaitReceipt broadcasts the tx and polls for receipt until timeout.
func sendTxAndWaitReceipt(t *testing.T, ctx context.Context, signed *types.Transaction) *types.Receipt {
	t.Helper()
	if err := client.SendTransaction(ctx, signed); err != nil {
		t.Fatalf("SendTransaction: %v", err)
	}
	deadline := time.Now().Add(blockedTxTimeout)
	for time.Now().Before(deadline) {
		receipt, err := client.TransactionReceipt(ctx, signed.Hash())
		if err == nil && receipt != nil {
			t.Logf("confirmed tx: %s", signed.Hash().Hex())
			return receipt
		}
		time.Sleep(1 * time.Second)
	}
	t.Logf("failed to confirm tx: %s", signed.Hash().Hex())
	return nil
}

// computeYenValue returns the token amount (in wei for native, raw units for ERC20) that equals yenAmount JPY.
func computeYenValue(nativeConfig config.NativeTokenConfig, tokenConfig config.TargetERC20Config, yenAmount uint64) (nativeValue, erc20Value big.Int) {
	oneE18 := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	decExp := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tokenConfig.Decimals)), nil)
	yen := new(big.Int).SetUint64(yenAmount)

	rateNative := nativeConfig.ToYenRate
	if rateNative <= 0 {
		nativeValue = *big.NewInt(1)
		erc20Value = *big.NewInt(1)
		return
	}
	// nativeValue = ceil(yenAmount * 1e18 / rate)
	fNative := new(big.Float).SetInt(new(big.Int).Mul(yen, oneE18))
	fNative.Quo(fNative, new(big.Float).SetFloat64(rateNative))
	fNative.Int(&nativeValue)
	nativeValue.Add(&nativeValue, big.NewInt(1))

	rateERC20 := tokenConfig.ToYenRate
	if rateERC20 <= 0 {
		erc20Value = *big.NewInt(1)
		return
	}
	// erc20Value = ceil(yenAmount * 10^decimals / rate)
	fERC20 := new(big.Float).SetInt(new(big.Int).Mul(yen, decExp))
	fERC20.Quo(fERC20, new(big.Float).SetFloat64(rateERC20))
	fERC20.Int(&erc20Value)
	erc20Value.Add(&erc20Value, big.NewInt(1))
	return
}

type ecdsaKey struct {
	key  *ecdsa.PrivateKey
	addr common.Address
}

func mustPrivKey(t *testing.T, hexKey string) *ecdsaKey {
	t.Helper()
	key, err := crypto.HexToECDSA(strings.TrimPrefix(hexKey, "0x"))
	if err != nil {
		t.Fatalf("HexToECDSA: %v", err)
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return &ecdsaKey{key: key, addr: addr}
}

func mustAddressFromPriv(t *testing.T, hexKey string) common.Address {
	t.Helper()
	return mustPrivKey(t, hexKey).addr
}

func mustAddressesFromPrivs(t *testing.T, hexKeys []string) []common.Address {
	t.Helper()
	out := make([]common.Address, len(hexKeys))
	for i, h := range hexKeys {
		out[i] = mustAddressFromPriv(t, h)
	}
	return out
}
