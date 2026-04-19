package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/core"
)

// Usage: go run txfilter/plugin_metadata_creator.go <plugin_path> [pubkey_path] [output_path] [version] [certificate_identity] [certificate_issuer]
//
// Examples:
//
//	With key:   go run txfilter/plugin_metadata_creator.go ./build/bin/suspicious_txfilter.so.bundle ./cosign.pub ./suspicious_txfilter.json 1.0.0
//	Keyless:    go run txfilter/plugin_metadata_creator.go ./build/bin/suspicious_txfilter.so.bundle - ./suspicious_txfilter.json 1.0.0 <certificate_identity> <certificate_issuer>
//
// Use "-" for pubkey_path to create keyless metadata. For keyless, certificate_identity and certificate_issuer are required (args 5 and 6).
func main() {
	// Default values
	pluginPath := filepath.Join("./", "build", "bin", core.PluginFileName)
	pubKeyPath := "cosign.pub"
	outputPath := "./suspicious_txfilter.json"
	version := "1.0.0"
	var certificateIdentity, certificateIssuer string

	// Parse positional arguments
	args := os.Args[1:]
	if len(args) > 0 {
		pluginPath = args[0]
	}
	if len(args) > 1 {
		pubKeyPath = args[1]
	}
	if len(args) > 2 {
		outputPath = args[2]
	}
	if len(args) > 3 {
		version = args[3]
	}
	if len(args) > 4 {
		certificateIdentity = args[4]
	}
	if len(args) > 5 {
		certificateIssuer = args[5]
	}

	keyless := pubKeyPath == "" || pubKeyPath == "-"
	if keyless && (certificateIdentity == "" || certificateIssuer == "") {
		log.Fatalf("Keyless mode requires certificate_identity and certificate_issuer (args 5 and 6)")
	}

	// Get absolute paths
	pluginAbsPath, err := filepath.Abs(pluginPath)
	if err != nil {
		log.Fatalf("Failed to get absolute path for plugin: %v", err)
	}
	outputAbsPath, err := filepath.Abs(outputPath)
	if err != nil {
		log.Fatalf("Failed to get absolute path for output: %v", err)
	}

	// Read bundle file (bundle file is plugin file + ".bundle")
	bundleData, err := os.ReadFile(pluginAbsPath)
	if err != nil {
		log.Fatalf("Failed to read bundle file %s: %v", pluginAbsPath, err)
	}

	bundleHex := hex.EncodeToString(bundleData)

	metadata := core.SuspiciousTxfilterPluginMetadata{
		Version:   version,
		BundleHex: bundleHex,
		Disable:   false,
	}

	if keyless {
		metadata.IsKeyless = true
		metadata.CertificateIdentity = &certificateIdentity
		metadata.CertificateIssuer = &certificateIssuer
	} else {
		pubKeyAbsPath, err := filepath.Abs(pubKeyPath)
		if err != nil {
			log.Fatalf("Failed to get absolute path for pubkey: %v", err)
		}
		pubKeyData, err := os.ReadFile(pubKeyAbsPath)
		if err != nil {
			log.Fatalf("Failed to read public key file %s: %v", pubKeyAbsPath, err)
		}
		pubKeyHex := hex.EncodeToString(pubKeyData)
		metadata.BundlePublicKeyHex = &pubKeyHex
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	// Determine output file path
	outputFile := filepath.Join(outputAbsPath)
	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		log.Fatalf("Failed to write JSON file: %v", err)
	}

	fmt.Printf("Plugin metadata created successfully at %s\n", outputFile)
}
