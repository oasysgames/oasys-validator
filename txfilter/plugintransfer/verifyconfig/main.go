package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/txfilter/plugintransfer/config"
)

// Usage: go run txfilter/plugintransfer/verifyconfig/main.go [config_json_path]
//
// Validates a PluginConfig JSON file. Exits 0 on success, 1 on validation or I/O error.
//
// Example:
//
//	go run txfilter/plugintransfer/verifyconfig/main.go ./plugintransfer.json
func main() {
	configPath := "./plugintransfer.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	absPath, err := filepath.Abs(configPath)
	if err != nil {
		log.Fatalf("Failed to get absolute path for config: %v", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		log.Fatalf("Failed to read config file %s: %v", absPath, err)
	}

	if err := VerifyFromJSON(data); err != nil {
		log.Fatalf("Config validation failed: %v", err)
	}

	fmt.Printf("Config validated successfully: %s\n", absPath)
}

const minMeasurementWindow = 1 * time.Hour

// ErrValidation is returned when config validation fails.
var ErrValidation = errors.New("config validation failed")

// VerifyFromJSON decodes the given JSON bytes into config.PluginConfig and runs all validations.
// Returns nil if valid, or an error describing the first validation failure.
func VerifyFromJSON(data []byte) error {
	var cfg config.PluginConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	return Verify(&cfg)
}

// Verify validates a loaded config.PluginConfig. Returns nil if valid.
func Verify(cfg *config.PluginConfig) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is nil", ErrValidation)
	}

	// version is not empty (meaningful version)
	if cfg.Version == 0 {
		return fmt.Errorf("%w: version must be greater than 0", ErrValidation)
	}

	// measurement window is larger than 1 min
	if cfg.MeasurementWindow <= 0 {
		return fmt.Errorf("%w: measurement_window must be set", ErrValidation)
	}
	if cfg.MeasurementWindow < minMeasurementWindow {
		return fmt.Errorf("%w: measurement_window must be larger than 1 minute (got %v)", ErrValidation, cfg.MeasurementWindow)
	}

	// warning threshold is smaller than block threshold (count and amount)
	t := &cfg.Threshold
	if t.WarningCountThreshold >= t.BlockCountThreshold {
		return fmt.Errorf("%w: warning_tx_count_threshold (%d) must be smaller than block_count_threshold (%d)",
			ErrValidation, t.WarningCountThreshold, t.BlockCountThreshold)
	}
	if t.WarningAmountThreshold >= t.BlockAmountThreshold {
		return fmt.Errorf("%w: warning_amount_threshold (%d) must be smaller than block_amount_threshold (%d)",
			ErrValidation, t.WarningAmountThreshold, t.BlockAmountThreshold)
	}

	// block thresholds must be positive (required for cache and filtering logic)
	if t.BlockCountThreshold == 0 {
		return fmt.Errorf("%w: block_count_threshold must be greater than 0", ErrValidation)
	}
	if t.BlockAmountThreshold == 0 {
		return fmt.Errorf("%w: block_amount_threshold must be greater than 0", ErrValidation)
	}

	// native token rate must be non-negative
	if cfg.NativeToken.ToYenRate < 0 {
		return fmt.Errorf("%w: native_token.to_yen_rate must be non-negative (got %f)", ErrValidation, cfg.NativeToken.ToYenRate)
	}

	// target ERC20s: valid address, decimals, and non-negative rate
	for i := range cfg.TargetERC20s {
		tok := &cfg.TargetERC20s[i]
		if tok.Address == (common.Address{}) {
			return fmt.Errorf("%w: target_erc20s[%d].address must not be zero", ErrValidation, i)
		}
		if tok.Decimals > 18 {
			return fmt.Errorf("%w: target_erc20s[%d].decimals must be at most 18 (got %d)", ErrValidation, i, tok.Decimals)
		}
		if tok.ToYenRate < 0 {
			return fmt.Errorf("%w: target_erc20s[%d].to_yen_rate must be non-negative (got %f)", ErrValidation, i, tok.ToYenRate)
		}
	}

	// Print the config
	fmt.Printf("config: %+v\n", cfg)

	return nil
}
