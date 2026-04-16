package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"plugin"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/internal/version"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// NOTE: Must run `make plugin-test` before running the tests. Otherwise, the tests will be skipped.
//       The command create the plugin files and metadata for testing in the `txfilter/testdata` directory.

var (
	errSkipTest = errors.New("skip test")
	projectRoot = "../"
	testConfig  = &params.ChainConfig{
		ChainID: params.AllDevChainProtocolChanges.ChainID,
		Oasys: &params.OasysConfig{
			Period: 15,
			Epoch:  5760,
		},
	}
)

func setupTestEnv(t *testing.T, metadataPath, pluginPath string) (chan struct{}, func(), error) {
	// Check if the metadata and plugin files exist, if not, skip the test
	if _, err := os.Stat(metadataPath); err != nil {
		return nil, nil, errSkipTest
	}
	if _, err := os.Stat(pluginPath); err != nil {
		return nil, nil, errSkipTest
	}

	// Create exit channel
	exitCh := make(chan struct{})

	// Setup test server
	localServer, listener, err := setupTestServer(t, metadataPath, pluginPath)
	if err != nil {
		return exitCh, nil, err
	}

	cleanup := func() {
		close(exitCh)
		localServer.Close()
		listener.Close()
	}

	return exitCh, cleanup, nil
}

// setupTestServer creates an HTTP test server on localhost:3030 to serve the metadata and plugin files.
func setupTestServer(t *testing.T, metadataPath, pluginPath string) (*httptest.Server, net.Listener, error) {
	// Read the metadata JSON from root directory
	metadataData, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	// Read the plugin .so file from root directory
	pluginData, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("Failed to read plugin file: %v", err)
	}

	// Create a test HTTP server to serve the metadata and plugin
	mux := http.NewServeMux()
	mux.HandleFunc("/suspicious_txfilter.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(metadataData)
	})
	mux.HandleFunc("/suspicious_txfilter.so", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(pluginData)
	})

	// Create a test HTTP server to serve the metadata and plugin
	localhostServer := httptest.NewUnstartedServer(mux)
	localhostServer.Listener.Close() // Close the auto-created listener

	// Try to listen on localhost:3030
	listener, err := net.Listen("tcp", "localhost:3030")
	if err != nil {
		t.Fatalf("Cannot bind to localhost:3030 (may be in use): %v", err)
	}

	localhostServer.Listener = listener
	localhostServer.Start()

	return localhostServer, listener, nil
}

func TestNewSuspiciousTxfilter(t *testing.T) {
	var (
		tmpDir       = t.TempDir() // Create a temporary directory for the test
		metadataPath = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v1.json")
		pluginPath   = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v1.so")
	)

	exitCh, cleanup, err := setupTestEnv(t, metadataPath, pluginPath)
	if err != nil {
		if errors.Is(err, errSkipTest) {
			t.Skipf("Skipping test: %v", err)
		}
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cleanup()

	// Now create the SuspiciousTxfilter
	filter, err := NewSuspiciousTxfilter(testConfig, tmpDir, exitCh)
	if err != nil {
		t.Fatalf("Failed to create SuspiciousTxfilter: %v", err)
	}

	// Wait a bit for the background goroutine to download and load the plugin
	time.Sleep(2 * time.Second)

	// Verify the plugin is ready
	if !filter.IsReady() {
		t.Errorf("Plugin is not ready")
	}

	// Verify that metadata was loaded
	metadata := filter.metadata.Load()
	if metadata == nil {
		t.Fatal("Metadata was not loaded")
	}

	// Verify that plugin file was downloaded
	pluginFilePath := filepath.Join(tmpDir, PluginFileName)
	if _, err := os.Stat(pluginFilePath); os.IsNotExist(err) {
		t.Error("Plugin file was not downloaded")
	}
}

func TestSuspiciousTxfilter_reloadPlugin(t *testing.T) {
	// Host the new plugin files
	var (
		newMetadataPath = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v2.json")
		newPluginPath   = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v2.so")
	)
	_, cleanup, err := setupTestEnv(t, newMetadataPath, newPluginPath)
	if err != nil {
		if errors.Is(err, errSkipTest) {
			t.Skipf("Skipping test: %v", err)
		}
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cleanup()

	var (
		tmpDir     = t.TempDir() // Create a temporary directory for the test
		pluginPath = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v1.so")
	)

	// Manually create the SuspiciousTxfilter
	filter := &SuspiciousTxfilter{
		datadir:  tmpDir,
		config:   testConfig,
		metadata: atomic.Pointer[SuspiciousTxfilterPluginMetadata]{},
		client:   &http.Client{},
	}
	p, err := plugin.Open(pluginPath)
	if err != nil {
		t.Fatalf("Failed to open plugin: %v", err)
	}
	filter.plugin.Store(p)
	bundlePublicKeyHex := "1234567890"
	metadata := &SuspiciousTxfilterPluginMetadata{
		Version:            "1.0.0",
		BundleHex:          "1234567890",
		IsKeyless:          false,
		BundlePublicKeyHex: &bundlePublicKeyHex,
		Disable:            false,
	}
	filter.metadata.Store(metadata)

	// Reload the plugin
	reload, err := filter.reloadPlugin()
	if err != nil {
		t.Fatalf("Failed to reload plugin: %v", err)
	}
	if !reload {
		t.Errorf("Plugin was not reloaded")
	}

	// Verify the plugin is ready
	if !filter.IsReady() {
		t.Errorf("Plugin is not ready")
	}
}

func TestSuspiciousTxfilter_FilterTransaction(t *testing.T) {
	var (
		tmpDir       = t.TempDir() // Create a temporary directory for the test
		pluginPath   = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v1.so")
		metadataPath = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v1.json")
	)

	exitCh, cleanup, err := setupTestEnv(t, metadataPath, pluginPath)
	if err != nil {
		if errors.Is(err, errSkipTest) {
			t.Skipf("Skipping test: %v", err)
		}
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cleanup()

	// Now create the SuspiciousTxfilter
	filter, err := NewSuspiciousTxfilter(testConfig, tmpDir, exitCh)
	if err != nil {
		t.Fatalf("Failed to create SuspiciousTxfilter: %v", err)
	}

	// Wait a bit for the background goroutine to download and load the plugin
	time.Sleep(2 * time.Second)

	// Test FilterTransaction
	to := common.HexToAddress("0x1234567890123456789012345678901234567890")
	msg := &Message{
		From:  common.HexToAddress("0x1234567890123456789012345678901234567890"),
		To:    &to,
		Value: big.NewInt(1000000000000000000),
	}
	logs := []*types.Log{
		{
			Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
			Topics: []common.Hash{
				common.HexToHash("0x1234567890123456789012345678901234567890"),
			},
			Data: []byte("0x1234567890123456789012345678901234567890"),
		},
	}
	isBlocked, reason, err := filter.FilterTransaction(common.Hash{}, msg, logs)
	if err != nil {
		t.Fatalf("Failed to filter transaction: %v", err)
	}
	if isBlocked {
		t.Errorf("Transaction was blocked")
	}
	if reason != "" {
		t.Errorf("Reason was not empty: %s", reason)
	}
}

func TestSuspiciousTxfilter_FilterTransaction_Blocked(t *testing.T) {
	var (
		tmpDir       = t.TempDir() // Create a temporary directory for the test
		pluginPath   = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v2.so")
		metadataPath = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v2.json")
	)

	exitCh, cleanup, err := setupTestEnv(t, metadataPath, pluginPath)
	if err != nil {
		if errors.Is(err, errSkipTest) {
			t.Skipf("Skipping test: %v", err)
		}
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cleanup()

	// Now create the SuspiciousTxfilter
	filter, err := NewSuspiciousTxfilter(testConfig, tmpDir, exitCh)
	if err != nil {
		t.Fatalf("Failed to create SuspiciousTxfilter: %v", err)
	}

	// Wait a bit for the background goroutine to download and load the plugin
	time.Sleep(2 * time.Second)

	// Test FilterTransaction with v2 plugin which blocks all transactions
	fromAddr := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	toAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	value := big.NewInt(1000000000000000000) // 1 ETH
	msg := &Message{
		From:  fromAddr,
		To:    &toAddr,
		Value: value,
	}
	logs := []*types.Log{
		{
			Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
			Topics: []common.Hash{
				common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"),
				common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222"),
			},
			Data: []byte("0x1234567890123456789012345678901234567890"),
		},
		{
			Address: common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
			Topics: []common.Hash{
				common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333"),
			},
			Data: []byte("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
		},
	}

	isBlocked, reason, err := filter.FilterTransaction(common.Hash{}, msg, logs)
	if err == nil {
		t.Fatalf("Expected error from blocked transaction, but got nil")
	}
	expectedErr := "isBlocked=true is ignored if the plugin returns an error"
	if err.Error() != expectedErr {
		t.Fatalf("Expected error '%s', but got %s", expectedErr, err.Error())
	}
	if !isBlocked {
		t.Errorf("Transaction should have been blocked")
	}
	if reason == "" {
		t.Fatalf("Reason should not be empty")
	}

	// Parse the reason JSON
	type LogEntry struct {
		Address string   `json:"address"`
		Topics  []string `json:"topics"`
		Data    string   `json:"data"`
	}
	type ReasonJSON struct {
		From  string     `json:"from"`
		To    string     `json:"to"`
		Value string     `json:"value"`
		Logs  []LogEntry `json:"logs"`
	}

	var reasonData ReasonJSON
	if err := json.Unmarshal([]byte(reason), &reasonData); err != nil {
		t.Fatalf("Failed to parse reason JSON: %v, reason: %s", err, reason)
	}

	// Verify from address
	if reasonData.From != fromAddr.Hex() {
		t.Errorf("From address mismatch: expected %s, got %s", fromAddr.Hex(), reasonData.From)
	}

	// Verify to address
	if reasonData.To != toAddr.Hex() {
		t.Errorf("To address mismatch: expected %s, got %s", toAddr.Hex(), reasonData.To)
	}

	// Verify value
	expectedValueHex := hexutil.Encode(math.PaddedBigBytes(value, 32))
	if reasonData.Value != expectedValueHex {
		t.Errorf("Value mismatch: expected %s, got %s", expectedValueHex, reasonData.Value)
	}

	// Verify logs
	if len(reasonData.Logs) != len(logs) {
		t.Fatalf("Log count mismatch: expected %d, got %d", len(logs), len(reasonData.Logs))
	}

	for i, log := range logs {
		reasonLog := reasonData.Logs[i]

		// Verify address
		if reasonLog.Address != log.Address.Hex() {
			t.Errorf("Log[%d] address mismatch: expected %s, got %s", i, log.Address.Hex(), reasonLog.Address)
		}

		// Verify topics
		if len(reasonLog.Topics) != len(log.Topics) {
			t.Errorf("Log[%d] topic count mismatch: expected %d, got %d", i, len(log.Topics), len(reasonLog.Topics))
		} else {
			for j, topic := range log.Topics {
				if reasonLog.Topics[j] != topic.Hex() {
					t.Errorf("Log[%d] topic[%d] mismatch: expected %s, got %s", i, j, topic.Hex(), reasonLog.Topics[j])
				}
			}
		}

		// Verify data
		expectedDataHex := hexutil.Encode(log.Data[:])
		if reasonLog.Data != expectedDataHex {
			t.Errorf("Log[%d] data mismatch: expected %s, got %s", i, expectedDataHex, reasonLog.Data)
		}
	}
}

func TestSuspiciousTxfilter_buildPluginURL(t *testing.T) {
	filter := &SuspiciousTxfilter{
		config: &params.ChainConfig{
			ChainID: big.NewInt(params.OasysMainnetChainConfig.ChainID.Int64()),
		},
	}
	url := filter.buildPluginURL(PluginFileName)
	expectedURL := fmt.Sprintf("https://cdn.mainnet.oasys.games/suspicious_txfilter/%s/suspicious_txfilter_%s_%s.so", version.WithMeta, runtime.GOOS, runtime.GOARCH)
	if url != expectedURL {
		t.Errorf("Expected URL: %s, got: %s", expectedURL, url)
	}
}
