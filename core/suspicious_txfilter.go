package core

import (
	"crypto"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"plugin"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/log"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	sigverify "github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

const (
	// The default interval to reload the plugin
	DefaultPluginReloadInterval = 1 * time.Hour

	// The suspicious txfilter plugin file name
	PluginFileName = "suspicious_txfilter.so"

	// The suspicious txfilter plugin metadata file name
	PluginMetadataFileName = "suspicious_txfilter.json"

	pluginFunctionName       = "FilterTransaction"
	pluginFuncNameGetVersion = "Version"
)

var (
	// Global instance of the suspicious tx filter.
	// This value is initialized in the `miner` package.
	//
	// We keep this as a global variable to avoid passing the instance through
	// multiple layers and changing many interfaces. Modifying those interfaces
	// would significantly increase the merge conflict effort.
	SuspiciousTxfilterGlobal *SuspiciousTxfilter
)

type SuspiciousTxfilterPluginMetadata struct {
	Version            string `json:"version"`
	BundleHex          string `json:"bundle_hex"`
	BundlePublicKeyHex string `json:"bundle_public_key_hex"`
	Disable            bool   `json:"disable"`
}

type SuspiciousTxfilter struct {
	datadir string
	config  *params.ChainConfig
	exitCh  chan struct{}

	plugin   atomic.Pointer[plugin.Plugin]
	metadata atomic.Pointer[SuspiciousTxfilterPluginMetadata]

	verifier *sigverify.Verifier
}

func NewSuspiciousTxfilter(config *params.ChainConfig, datadir string, exitCh chan struct{}) (*SuspiciousTxfilter, error) {
	if config.Oasys == nil {
		return nil, fmt.Errorf("suspicious tx filter is only supported on oasys chain")
	}

	b := &SuspiciousTxfilter{
		datadir: datadir,
		config:  config,
		exitCh:  exitCh,
	}

	if _, _, err := b.fetchPluginMetadata(); err != nil {
		return nil, fmt.Errorf("failed to download plugin metadata: %w", err)
	}

	// Do background loading of the plugin
	go func() {
		// Try to load existing plugin, fetch if missing or invalid
		pluginPath := b.pluginPath()
		if _, err := os.Stat(pluginPath); os.IsNotExist(err) || b.loadPlugin(true) != nil {
			if err := b.fetchPlugin(); err != nil {
				log.Error("Failed to download suspicious txfilter plugin", "err", err)
			}
			if err := b.loadPlugin(true); err != nil {
				log.Error("Failed to load suspicious txfilter plugin", "err", err)
			}
		}

		go b.startReloadLoop(DefaultPluginReloadInterval)
	}()

	return b, nil
}

func (b *SuspiciousTxfilter) IsReady() bool {
	if metadata := b.metadata.Load(); metadata == nil || metadata.Disable || b.plugin.Load() == nil {
		return false
	}
	return true
}

func (b *SuspiciousTxfilter) VerifyPluginVersion(plugin *plugin.Plugin) error {
	metadata := b.metadata.Load()
	if metadata == nil || metadata.Version == "" {
		return fmt.Errorf("plugin metadata not found")
	}

	f, err := plugin.Lookup(pluginFuncNameGetVersion)
	if err != nil {
		return fmt.Errorf("failed to lookup plugin function: %w", err)
	}
	version, ok := f.(func() string)
	if !ok {
		return fmt.Errorf("plugin function has incorrect signature")
	}

	pluginVersion := version()
	if pluginVersion != metadata.Version {
		return fmt.Errorf("plugin version mismatch: %s != %s", pluginVersion, metadata.Version)
	}
	return nil
}

func (b *SuspiciousTxfilter) FilterTransaction(msg *Message, logs []*types.Log) (isBlocked bool, reason string, err error) {
	// Don't filter if the plugin is disabled
	if metadata := b.metadata.Load(); metadata == nil || metadata.Disable {
		return false, "", nil
	}

	// Skip filtering if the plugin is not loaded
	plugin := b.plugin.Load()
	if plugin == nil {
		return false, "", fmt.Errorf("plugin not loaded")
	}

	// Copy data to call the plugin function
	var (
		from, to   common.Address
		value      [32]byte
		copiedLogs = make([]types.Log, len(logs))
	)
	from = msg.From
	copy(to[:], msg.To[:])
	copy(value[:], math.PaddedBigBytes(msg.Value, 32))
	for i, log := range logs {
		copy(copiedLogs[i].Address[:], log.Address[:])
		copiedLogs[i].Topics = make([]common.Hash, len(log.Topics))
		for j, topic := range log.Topics {
			copy(copiedLogs[i].Topics[j][:], topic[:])
		}
		copiedLogs[i].Data = make([]byte, len(log.Data))
		copy(copiedLogs[i].Data, log.Data)
	}

	// Call the plugin function
	f, err := plugin.Lookup(pluginFunctionName)
	if err != nil {
		return false, "", fmt.Errorf("failed to lookup plugin function: %w", err)
	}
	process, ok := f.(func(common.Address, common.Address, [32]byte, []types.Log) (bool, string, error))
	if !ok {
		return false, "", fmt.Errorf("plugin function has incorrect signature")
	}
	return process(from, to, value, copiedLogs)
}

func (b *SuspiciousTxfilter) fetchPlugin() error {
	url := b.pluginDownloadURL()
	body, err := fetch(url)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}
	defer body.Close()

	pluginPath := b.pluginPath()
	file, err := os.OpenFile(pluginPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644) // Overwrite the plugin file
	if err != nil {
		return fmt.Errorf("failed to create plugin file: %w", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, body); err != nil {
		return fmt.Errorf("failed to copy plugin body to file: %w", err)
	}
	return nil
}

func (b *SuspiciousTxfilter) fetchPluginMetadata() (isNewPlugin bool, isNewPubKey bool, err error) {
	metadata := b.metadata.Load()
	if metadata == nil {
		// Initialize with default values
		metadata = &SuspiciousTxfilterPluginMetadata{}
	}

	body, err := fetch(b.pluginMetadataDownloadURL())
	if err != nil {
		return false, false, fmt.Errorf("failed to download plugin metadata: %w", err)
	}
	defer body.Close()

	var (
		oldVersion   = metadata.Version
		oldPubKeyHex = metadata.BundlePublicKeyHex
	)
	if err = json.NewDecoder(body).Decode(metadata); err != nil {
		return false, false, fmt.Errorf("failed to unmarshal plugin metadata: %w", err)
	}
	if metadata.Version != oldVersion {
		isNewPlugin = true
	}
	if metadata.BundlePublicKeyHex != oldPubKeyHex {
		isNewPubKey = true
	}
	b.metadata.Store(metadata)
	return
}

func (b *SuspiciousTxfilter) startReloadLoop(reloadInterval time.Duration) {
	log.Info("Starting suspicious txfilter reload loop", "reloadInterval", reloadInterval)

	timer := time.NewTimer(reloadInterval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			reload, err := b.reloadPlugin()
			if err != nil {
				log.Warn("Failed to reload suspicious txfilter plugin", "err", err)
			}
			if reload {
				log.Info("Reloaded suspicious txfilter plugin", "version", b.metadata.Load().Version)
			}
		case <-b.exitCh:
			log.Info("Stop suspicious txfilter reload loop", "exitCh", b.exitCh)
			return
		}
	}
}

func (b *SuspiciousTxfilter) loadPlugin(isNewPubKey bool) error {
	bundleData, err := hex.DecodeString(b.metadata.Load().BundleHex)
	if err != nil {
		return fmt.Errorf("failed to decode bundle hex: %w", err)
	}

	var bundle bundle.Bundle
	if err = bundle.UnmarshalJSON(bundleData); err != nil {
		return fmt.Errorf("failed to unmarshal bundle: %w", err)
	}

	if isNewPubKey {
		if err = b.updateVerifier(); err != nil {
			return err
		}
	}

	if err = b.verifyPlugin(bundle); err != nil {
		return err
	}

	loadedPlugin, err := plugin.Open(b.pluginPath())
	if err != nil {
		return fmt.Errorf("failed to open plugin: %w", err)
	}

	if err = b.VerifyPluginVersion(loadedPlugin); err != nil {
		b.plugin.Store(nil)
		return fmt.Errorf("failed to verify plugin version: %w", err)
	}

	b.plugin.Store(loadedPlugin)
	return nil
}

func (b *SuspiciousTxfilter) updateVerifier() error {
	pubKeyData, err := hex.DecodeString(b.metadata.Load().BundlePublicKeyHex)
	if err != nil {
		return fmt.Errorf("failed to decode bundle public key hex: %w", err)
	}

	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(pubKeyData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal bundle public key: %w", err)
	}

	trustedMaterial := trustedPublicKeyMaterial(pubKey)
	if b.verifier, err = sigverify.NewVerifier(trustedMaterial, sigverify.WithNoObserverTimestamps()); err != nil {
		return fmt.Errorf("failed to create verifier: %w", err)
	}
	return nil
}

func (b *SuspiciousTxfilter) verifyPlugin(bundle bundle.Bundle) error {
	if b.verifier == nil {
		return fmt.Errorf("verifier not initialized")
	}

	pluginPath := b.pluginPath()
	file, err := os.Open(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to open plugin file: %w", err)
	}
	defer file.Close()

	artifactPolicy := sigverify.WithArtifact(file)
	if _, err := b.verifier.Verify(&bundle, sigverify.NewPolicy(artifactPolicy, sigverify.WithKey())); err != nil {
		return fmt.Errorf("failed to verify bundle: %w", err)
	}
	return nil
}

func (b *SuspiciousTxfilter) reloadPlugin() (reload bool, err error) {
	// Download the plugin metadata
	isNewPlugin, isNewPubKey, err := b.fetchPluginMetadata()
	if err != nil {
		err = fmt.Errorf("failed to download plugin metadata: %w", err)
		return
	}
	if !isNewPlugin {
		return
	}
	if err = b.fetchPlugin(); err != nil {
		err = fmt.Errorf("failed to download plugin: %w", err)
		return
	}
	if err = b.loadPlugin(isNewPubKey); err != nil {
		err = fmt.Errorf("failed to load plugin: %w", err)
		return
	}
	reload = true
	return
}

func (b *SuspiciousTxfilter) pluginPath() string {
	return path.Join(b.datadir, PluginFileName)
}

func (b *SuspiciousTxfilter) buildPluginURL(filename string) string {
	// For mainnet and testnet
	if b.config.ChainID.Cmp(params.OasysMainnetChainConfig.ChainID) == 0 || b.config.ChainID.Cmp(params.OasysTestnetChainConfig.ChainID) == 0 {
		var (
			fileExt  = filepath.Ext(filename)                // e.g., ".so" or ".json"
			fileName = strings.TrimSuffix(filename, fileExt) // e.g., "suspicious_txfilter"
			network  = "mainnet"
			osName   = runtime.GOOS
			osArch   = runtime.GOARCH
		)
		if b.config.ChainID.Cmp(params.OasysTestnetChainConfig.ChainID) == 0 {
			network = "testnet"
		}
		return fmt.Sprintf("https://cdn.%s.oasys.games/suspicious_txfilter/%s_%s_%s%s", network, fileName, osName, osArch, fileExt)
	}

	// For the default case, assume it is oasys-private-l1.
	var (
		host = "pluginserver" // From the `pluginserver` service in `oasys-private-l1`
		port = "3030"
		ip   string
	)
	// Lookup the IP address, if failed, give up and use the host name.
	ips, err := net.LookupIP(host)
	if err != nil {
		log.Error("Failed to lookup plugin server IP", "host", host, "err", err)
		ip = host
	} else if len(ips) == 0 {
		log.Error("No IP found for plugin server", "host", host)
		ip = host
	} else {
		ip = ips[0].String()
	}
	return fmt.Sprintf("http://%s:%s/%s", ip, port, filename)
}

func (b *SuspiciousTxfilter) pluginDownloadURL() string {
	return b.buildPluginURL(PluginFileName)
}

func (b *SuspiciousTxfilter) pluginMetadataDownloadURL() string {
	return b.buildPluginURL(PluginMetadataFileName)
}

func fetch(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: url: %s, err: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download error: status %d, url: %s", resp.StatusCode, url)
	}
	return resp.Body, nil
}

type nonExpiringVerifier struct {
	signature.Verifier
}

func (*nonExpiringVerifier) ValidAtTime(_ time.Time) bool {
	return true
}

func trustedPublicKeyMaterial(pk crypto.PublicKey) root.TrustedMaterial {
	return root.NewTrustedPublicKeyMaterial(func(_ string) (root.TimeConstrainedVerifier, error) {
		verifier, err := signature.LoadECDSAVerifier(pk.(*ecdsa.PublicKey), crypto.SHA256)
		if err != nil {
			return nil, err
		}
		return &nonExpiringVerifier{verifier}, nil
	})
}
