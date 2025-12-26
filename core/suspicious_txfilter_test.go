package core

import (
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"plugin"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

var (
	errSkipTest = errors.New("skip test")
	projectRoot = "../"
	testConfig  = &params.ChainConfig{
		ChainID: big.NewInt(12345),
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

// setupTestServer creates an HTTP test server on localhost:8080 to serve the metadata and plugin files.
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

	// Try to listen on localhost:8080
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		t.Skipf("Cannot bind to localhost:8080 (may be in use): %v", err)
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
	var (
		tmpDir     = t.TempDir() // Create a temporary directory for the test
		pluginPath = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v1.so")
	)

	// Manually create the SuspiciousTxfilter
	filter := &SuspiciousTxfilter{
		datadir:  tmpDir,
		config:   testConfig,
		metadata: atomic.Pointer[SuspiciousTxfilterPluginMetadata]{},
	}
	p, err := plugin.Open(pluginPath)
	if err != nil {
		t.Fatalf("Failed to open plugin: %v", err)
	}
	filter.plugin.Store(p)
	metadata := &SuspiciousTxfilterPluginMetadata{
		Version:            "1.0.0",
		BundleHex:          "1234567890",
		BundlePublicKeyHex: "1234567890",
		Disable:            false,
	}
	filter.metadata.Store(metadata)

	// Host the new plugin files
	var (
		metadataPath2 = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v2.json")
		pluginPath2   = filepath.Join(projectRoot, "txfilter", "testdata", "suspicious_txfilter-v2.so")
	)
	_, cleanup, err := setupTestEnv(t, metadataPath2, pluginPath2)
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cleanup()

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
	isBlocked, reason, err := filter.FilterTransaction(msg, logs)
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
