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
	txfilterlog "github.com/ethereum/go-ethereum/txfilter/log"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	sigverify "github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/version"
	"github.com/ethereum/go-ethereum/params"
)

const (
	// The default interval to reload the plugin
	DefaultPluginReloadInterval = 1 * time.Hour

	// The suspicious txfilter plugin file name
	PluginFileName = "suspicious_txfilter.so"

	// The suspicious txfilter plugin metadata file name
	PluginMetadataFileName = "suspicious_txfilter.json"

	// Exported symbol names in the plugin .so.
	pluginSymbolName           = "Plugin"
	pluginBuildFingerprintName = "BuildFingerprint"
)

var (
	// Global instance of the suspicious tx filter.
	// This value is initialized in the `miner` package.
	//
	// We keep this as a global variable to avoid passing the instance through
	// multiple layers and changing many interfaces. Modifying those interfaces
	// would significantly increase the merge conflict effort.
	SuspiciousTxfilterGlobal *SuspiciousTxfilter

	// pluginBuildFingerprint is set by -ldflags in build/ci.go doInstall().
	// It is compared against the plugin's BuildFingerprint to verify build compatibility.
	pluginBuildFingerprint string
)

// SuspiciousTxfilterPlugin is the interface that a loaded plugin must implement.
type SuspiciousTxfilterPlugin interface {
	// Version returns the plugin version string, used to verify compatibility with the host.
	Version() string
	// Clear drops runtime state so the host can load a new plugin instance; called before plugin reload.
	Clear() error
	// FilterTransaction decides whether to block the transaction.
	FilterTransaction(txhash common.Hash, from, to common.Address, value [32]byte, logs []txfilterlog.Log) (isBlocked bool, reason string, err error)
}

type SuspiciousTxfilterPluginMetadata struct {
	Version             string  `json:"version"`
	BundleHex           string  `json:"bundle_hex"`
	IsKeyless           bool    `json:"is_keyless"`
	CertificateIdentity *string `json:"certificate_identity,omitempty"`  // Required for keyless
	CertificateIssuer   *string `json:"certificate_issuer,omitempty"`    // Required for keyless
	BundlePublicKeyHex  *string `json:"bundle_public_key_hex,omitempty"` // Required for non-keyless
	Disable             bool    `json:"disable"`
}

type SuspiciousTxfilter struct {
	datadir string
	config  *params.ChainConfig
	exitCh  chan struct{}

	plugin   atomic.Pointer[plugin.Plugin]
	metadata atomic.Pointer[SuspiciousTxfilterPluginMetadata]

	verifier *sigverify.Verifier
	client   *http.Client
}

func NewSuspiciousTxfilter(config *params.ChainConfig, datadir string, exitCh chan struct{}) (*SuspiciousTxfilter, error) {
	if config.Oasys == nil {
		return nil, fmt.Errorf("suspicious tx filter is only supported on oasys chain")
	}

	s := &SuspiciousTxfilter{
		datadir: datadir,
		config:  config,
		exitCh:  exitCh,
		client:  &http.Client{Timeout: 30 * time.Second},
	}

	log.Info("Fetching suspicious txfilter plugin metadata", "url", s.pluginMetadataDownloadURL())
	if _, err := s.fetchPluginMetadata(); err != nil {
		return nil, fmt.Errorf("failed to download plugin metadata: %w", err)
	}
	log.Info("Suspicious txfilter plugin metadata loaded",
		"version", s.metadata.Load().Version, "disable", s.metadata.Load().Disable)

	// Do background loading of the plugin
	go func() {
		// Start the reload loop even if the plugin is not loaded successfully.
		defer s.startReloadLoop(DefaultPluginReloadInterval)

		// Try to load existing plugin, fetch if missing or invalid
		pluginPath := s.pluginPath()
		if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
			log.Info("Plugin not found locally", "path", pluginPath)
		} else if err := s.loadPlugin(); err != nil {
			log.Warn("Failed to load existing plugin, re-downloading", "path", pluginPath, "err", err)
		} else {
			return // loaded successfully
		}
		if err := s.fetchPlugin(); err != nil {
			log.Error("Failed to download suspicious txfilter plugin", "err", err)
		} else if err := s.loadPlugin(); err != nil {
			log.Error("Failed to load suspicious txfilter plugin", "err", err)
		}
	}()

	return s, nil
}

func (s *SuspiciousTxfilter) IsReady() bool {
	if metadata := s.metadata.Load(); metadata == nil || metadata.Disable || s.plugin.Load() == nil {
		return false
	}
	return true
}

// verifyPluginIntegrity verifies that the plugin .so file has not been tampered
// with by checking its Cosign signature bundle against the expected identity.
func (s *SuspiciousTxfilter) verifyPluginIntegrity(bundle bundle.Bundle, metadata *SuspiciousTxfilterPluginMetadata) error {
	if s.verifier == nil {
		return fmt.Errorf("verifier not initialized")
	}

	pluginPath := s.pluginPath()
	file, err := os.Open(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to open plugin file: %w", err)
	}
	defer file.Close()

	artifactPolicy := sigverify.WithArtifact(file)

	if metadata.IsKeyless {
		// Execute the verification
		// - WithArtifact: The artifact to verify
		// - WithCertificateIdentity: The identity of the certificate that signed the artifact
		if _, err := s.verifier.Verify(&bundle, sigverify.NewPolicy(
			artifactPolicy,
			sigverify.WithCertificateIdentity(sigverify.CertificateIdentity{
				SubjectAlternativeName: sigverify.SubjectAlternativeNameMatcher{
					SubjectAlternativeName: *metadata.CertificateIdentity,
				},
				Issuer: sigverify.IssuerMatcher{
					Issuer: *metadata.CertificateIssuer,
				},
			}),
		)); err != nil {
			return fmt.Errorf("failed to verify bundle: %w", err)
		}
		return nil
	}

	if _, err := s.verifier.Verify(&bundle, sigverify.NewPolicy(artifactPolicy, sigverify.WithKey())); err != nil {
		return fmt.Errorf("failed to verify bundle: %w", err)
	}
	return nil
}

// verifyPluginBuild checks that the plugin was built with the same go.mod,
// go.sum, Go version, and target OS/ARCH as the main binary.
func verifyPluginBuild(p *plugin.Plugin) error {
	if pluginBuildFingerprint == "" {
		return nil // development build without fingerprint
	}
	sym, err := p.Lookup(pluginBuildFingerprintName)
	if err != nil {
		log.Warn("Plugin does not export BuildFingerprint, skipping build compatibility check")
		return nil
	}
	pluginFP, ok := sym.(*string)
	if !ok {
		return fmt.Errorf("plugin BuildFingerprint has unexpected type: %T", sym)
	}
	if *pluginFP != pluginBuildFingerprint {
		return fmt.Errorf("build fingerprint mismatch: host=%s plugin=%s (rebuild plugin with the same go.sum and Go version)", pluginBuildFingerprint, *pluginFP)
	}
	return nil
}

// verifyPluginVersion checks that the loaded plugin's version matches
// the version declared in the metadata downloaded from the CDN.
func (s *SuspiciousTxfilter) verifyPluginVersion(p *plugin.Plugin) error {
	metadata := s.metadata.Load()
	if metadata == nil || metadata.Version == "" {
		return fmt.Errorf("plugin metadata not found")
	}

	pluginVersion, err := pluginVersion(p)
	if err != nil {
		return fmt.Errorf("failed to get plugin version: %w", err)
	}
	if pluginVersion != metadata.Version {
		return fmt.Errorf("plugin version mismatch: %s != %s", pluginVersion, metadata.Version)
	}
	return nil
}

func (s *SuspiciousTxfilter) FilterTransaction(txhash common.Hash, msg *Message, logs []*types.Log) (isBlocked bool, reason string, err error) {
	// Don't filter if the plugin is disabled
	if metadata := s.metadata.Load(); metadata == nil || metadata.Disable {
		return false, "", nil
	}

	// Skip filtering if the plugin is not loaded
	plugin := s.plugin.Load()
	if plugin == nil {
		return false, "", fmt.Errorf("plugin not loaded")
	}
	impl, err := loadPluginImpl(plugin)
	if err != nil {
		return false, "", fmt.Errorf("failed to load plugin implementation: %w", err)
	}

	// Copy data to call the plugin function
	var (
		from, to   common.Address
		value      [32]byte
		copiedLogs = make([]txfilterlog.Log, len(logs))
	)
	from = msg.From
	if msg.To != nil {
		copy(to[:], msg.To[:])
	}
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

	return impl.FilterTransaction(txhash, from, to, value, copiedLogs)
}

func (s *SuspiciousTxfilter) fetchPlugin() error {
	url := s.pluginDownloadURL()
	log.Info("Downloading suspicious txfilter plugin", "url", url)
	body, err := s.fetch(url)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}
	defer body.Close()

	pluginPath := s.pluginPath()
	file, err := os.OpenFile(pluginPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644) // Overwrite the plugin file
	if err != nil {
		return fmt.Errorf("failed to create plugin file: %w", err)
	}

	// Clean up the plugin file if the copy fails
	failCopy := false
	defer func() {
		if err := file.Close(); err != nil {
			log.Error("failed to close plugin file", "path", pluginPath, "err", err)
			failCopy = true
		}
		if failCopy {
			os.Remove(pluginPath)
		}
	}()

	if _, err = io.Copy(file, body); err != nil {
		failCopy = true
		return fmt.Errorf("failed to copy plugin body to file: %w", err)
	}
	return nil
}

func (s *SuspiciousTxfilter) fetchPluginMetadata() (version string, err error) {
	metadata := s.metadata.Load()
	if metadata == nil {
		// Initialize with default values
		metadata = &SuspiciousTxfilterPluginMetadata{}
	}

	body, err := s.fetch(s.pluginMetadataDownloadURL())
	if err != nil {
		return "", fmt.Errorf("failed to download plugin metadata: %w", err)
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(metadata); err != nil {
		return "", fmt.Errorf("failed to unmarshal plugin metadata: %w", err)
	}

	if metadata.IsKeyless {
		if metadata.CertificateIdentity == nil {
			return "", fmt.Errorf("certificate_identity is required for keyless plugin verification")
		}
		if metadata.CertificateIssuer == nil {
			return "", fmt.Errorf("certificate_issuer is required for keyless plugin verification")
		}
	} else {
		if metadata.BundlePublicKeyHex == nil {
			return "", fmt.Errorf("public key is required for non-keyless plugin verification")
		}
	}

	s.metadata.Store(metadata)
	return metadata.Version, nil
}

func (s *SuspiciousTxfilter) startReloadLoop(reloadInterval time.Duration) {
	log.Info("Starting suspicious txfilter reload loop", "reloadInterval", reloadInterval)

	timer := time.NewTimer(reloadInterval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			reload, err := s.reloadPlugin()
			if err != nil {
				log.Warn("Failed to reload suspicious txfilter plugin", "err", err)
			}
			if reload {
				log.Info("Reloaded suspicious txfilter plugin", "version", s.metadata.Load().Version)
			}
			timer.Reset(reloadInterval)
		case <-s.exitCh:
			log.Info("Stop suspicious txfilter reload loop", "exitCh", s.exitCh)
			return
		}
	}
}

func (s *SuspiciousTxfilter) loadPlugin() error {
	pluginPath := s.pluginPath()
	metadata := s.metadata.Load()

	bundleData, err := hex.DecodeString(metadata.BundleHex)
	if err != nil {
		return fmt.Errorf("failed to decode bundle hex: %w", err)
	}

	var bundle bundle.Bundle
	if err = bundle.UnmarshalJSON(bundleData); err != nil {
		return fmt.Errorf("failed to unmarshal bundle: %w", err)
	}

	// It's ok to update the verifier every time, as loading new plugin is not frequent.
	if err = s.updateVerifier(metadata); err != nil {
		return err
	}

	log.Info("Verifying plugin integrity", "path", pluginPath)
	if err = s.verifyPluginIntegrity(bundle, metadata); err != nil {
		return err
	}

	log.Info("Opening plugin", "path", pluginPath)
	newPlugin, err := plugin.Open(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to open plugin: %w", err)
	}

	log.Info("Verifying plugin build compatibility")
	if err = verifyPluginBuild(newPlugin); err != nil {
		return fmt.Errorf("failed to verify plugin build compatibility: %w", err)
	}

	log.Info("Verifying plugin version", "expected", metadata.Version)
	if err = s.verifyPluginVersion(newPlugin); err != nil {
		return fmt.Errorf("failed to verify plugin version: %w", err)
	}

	log.Info("Plugin loaded successfully", "version", metadata.Version)

	oldPlugin := s.plugin.Load()

	// Replace or store the newly loaded plugin instance
	s.plugin.Store(newPlugin)

	// Clear the old plugin so it can release runtime outdated state
	if oldPlugin != nil {
		if impl, err := loadPluginImpl(oldPlugin); err == nil {
			if err = impl.Clear(); err != nil {
				log.Warn("failed to clear suspicious txfilter plugin", "err", err)
			}
		}
	}

	return nil
}

func (s *SuspiciousTxfilter) updateVerifier(metadata *SuspiciousTxfilterPluginMetadata) error {
	if metadata.IsKeyless {
		// Get the public TUF root from Sigstore
		trustedRoot, err := root.FetchTrustedRoot()
		if err != nil {
			return fmt.Errorf("failed to fetch trusted root: %w", err)
		}
		// Create the verifier
		// - WithSignedCertificateTimestamps(1): The certificate is registered in the real CT log
		// - WithTransparencyLog(1): Require at least one Rekor entry
		// - WithObserverTimestamps(1): The certificate is valid at the time of the Rekor entry timestamp
		if s.verifier, err = sigverify.NewVerifier(
			trustedRoot,
			sigverify.WithSignedCertificateTimestamps(1),
			sigverify.WithTransparencyLog(1),
			sigverify.WithObserverTimestamps(1),
		); err != nil {
			return fmt.Errorf("failed to create verifier: %w", err)
		}
		return nil
	}

	pubKeyData, err := hex.DecodeString(*metadata.BundlePublicKeyHex)
	if err != nil {
		return fmt.Errorf("failed to decode bundle public key hex: %w", err)
	}
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(pubKeyData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal bundle public key: %w", err)
	}

	trustedRoot := trustedPublicKeyMaterial(pubKey)
	if s.verifier, err = sigverify.NewVerifier(
		trustedRoot,
		sigverify.WithNoObserverTimestamps(),
	); err != nil {
		return fmt.Errorf("failed to create verifier: %w", err)
	}
	return nil
}

func (s *SuspiciousTxfilter) reloadPlugin() (reloaded bool, err error) {
	// Download the plugin metadata
	metadataVersion, err := s.fetchPluginMetadata()
	if err != nil {
		err = fmt.Errorf("failed to download plugin metadata: %w", err)
		return
	}

	// Skip reloading if the plugin version matches the metadata version
	if pluginVersion, err := pluginVersion(s.plugin.Load()); err == nil && pluginVersion == metadataVersion {
		return false, nil
	}

	if err = s.fetchPlugin(); err != nil {
		err = fmt.Errorf("failed to download plugin: %w", err)
		return
	}
	if err = s.loadPlugin(); err != nil {
		err = fmt.Errorf("failed to load plugin: %w", err)
		return
	}
	return true, nil
}

func (s *SuspiciousTxfilter) pluginPath() string {
	return path.Join(s.datadir, PluginFileName)
}

func (s *SuspiciousTxfilter) buildPluginURL(filename string) string {
	// For mainnet and testnet
	if s.config.ChainID.Cmp(params.OasysMainnetChainConfig.ChainID) == 0 || s.config.ChainID.Cmp(params.OasysTestnetChainConfig.ChainID) == 0 {
		var (
			fileExt  = filepath.Ext(filename)                // e.g., ".so" or ".json"
			fileName = strings.TrimSuffix(filename, fileExt) // e.g., "suspicious_txfilter"
			network  = "mainnet"
			osName   = runtime.GOOS
			osArch   = runtime.GOARCH
		)
		if s.config.ChainID.Cmp(params.OasysTestnetChainConfig.ChainID) == 0 {
			network = "testnet"
		}
		return fmt.Sprintf("https://cdn.%s.oasys.games/suspicious_txfilter/%s/%s_%s_%s%s", network, version.WithMeta, fileName, osName, osArch, fileExt)
	}

	// For testing
	if s.config.ChainID.Cmp(params.AllDevChainProtocolChanges.ChainID) == 0 {
		return fmt.Sprintf("http://localhost:3030/%s", filename)
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

func (s *SuspiciousTxfilter) pluginDownloadURL() string {
	return s.buildPluginURL(PluginFileName)
}

func (s *SuspiciousTxfilter) pluginMetadataDownloadURL() string {
	return s.buildPluginURL(PluginMetadataFileName)
}

func (s *SuspiciousTxfilter) fetch(url string) (io.ReadCloser, error) {
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: url: %s, err: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download error: status %d, url: %s", resp.StatusCode, url)
	}
	return resp.Body, nil
}

func pluginVersion(p *plugin.Plugin) (string, error) {
	if p == nil {
		return "", fmt.Errorf("plugin is nil")
	}
	impl, err := loadPluginImpl(p)
	if err != nil {
		return "", err
	}
	return impl.Version(), nil
}

func loadPluginImpl(p *plugin.Plugin) (SuspiciousTxfilterPlugin, error) {
	symbol, err := p.Lookup(pluginSymbolName)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup plugin symbol: %w", err)
	}
	if impl, ok := symbol.(SuspiciousTxfilterPlugin); ok {
		return impl, nil
	}
	return nil, fmt.Errorf("plugin symbol has incorrect type: %T", symbol)
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
