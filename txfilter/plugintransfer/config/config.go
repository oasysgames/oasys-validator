package config

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// PluginConfig is loaded from remote JSON and controls all filter behavior.
type PluginConfig struct {
	Version           uint64                  `json:"version"`
	Whitelists        map[common.Address]bool `json:"whitelists"`
	MeasurementWindow time.Duration           `json:"measurement_window"`
	Threshold         ThresholdConfig         `json:"threshold"`
	NativeToken       NativeTokenConfig       `json:"native_token"`
	TargetERC20s      []TargetERC20Config     `json:"target_erc20s"`
	Disabled          bool                    `json:"disabled"`
}

type NativeTokenConfig struct {
	ToYenRate float64 `json:"to_yen_rate"`
}

type TargetERC20Config struct {
	Address   common.Address `json:"address"`
	Decimals  uint8          `json:"decimals"`
	ToYenRate float64        `json:"to_yen_rate"`
}

// Ok, if the amount is same as the threshold
type ThresholdConfig struct {
	WarningCountThreshold  uint   `json:"warning_tx_count_threshold"`
	BlockCountThreshold    uint   `json:"block_count_threshold"`
	WarningAmountThreshold uint64 `json:"warning_amount_threshold"`
	BlockAmountThreshold   uint64 `json:"block_amount_threshold"`
}
