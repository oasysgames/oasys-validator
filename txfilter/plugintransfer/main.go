package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

var (
	transferEventTopic = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
	configCache        = ConfigCache{
		Config:    PluginConfig{},
		updatedAt: time.Time{},
		ttl:       1 * time.Hour,
		client:    http.Client{Timeout: 30 * time.Second},
	}
	emptyHash common.Hash

	countedTxs *lrucache
)

// Set config URL at build time using -ldflags "-X main.configURL=https://cdn.oasys.games/suspicious_txfilter/plugintransfer.json"
var configURL = "http://localhost:3030/plugintransfer.json"

// Set version at build time using -ldflags "-X main.version=1.0.0"
var version = "1.0.0"

func Version() string {
	return version
}

func FilterTransaction(txhash common.Hash, from, to common.Address, value [32]byte, logs []types.Log) (isBlocked bool, reason string, err error) {
	// Update config if it's expired
	if configCache.isExpired() {
		if err := configCache.update(); err != nil {
			if configCache.isEmptyConfig() {
				return block(fmt.Sprintf("config is empty: %s", err))
			}
			// Just log the error, keep using the old config
			log.Error("failed to fetch config", "error", err, "url", configURL)
		}
	}

	// Skip if the plugin is disabled, or whitelisted, or already counted
	if configCache.Config.Disabled || configCache.isWhitelisted(from) || countedTxs.contains(txhash) {
		return allow()
	}

	// initialize accumulated amount by native token amount
	accumulatedAmount := configCache.toYen(nil, value)

	for _, log := range logs {
		// Skip if the log is not a transfer event
		if len(log.Topics) != 3 || log.Topics[0] != transferEventTopic {
			continue
		}
		// Skip if the log is not a target ERC20
		target, ok := configCache.isTargetERC20(log.Address)
		if !ok {
			continue
		}
		// Add accumulated amount by ERC20 token amount
		accumulatedAmount += configCache.toYen(target, log.Topics[2])
	}

	// Skip if the accumulated amount is zero or smaller than 1 yen
	if accumulatedAmount == 0 {
		return allow()
	}

	// Check the count and amount thresholds
	blocks, reason := isOverThreshold(&configCache.Config, countedTxs, accumulatedAmount)
	if blocks {
		return block(reason)
	}

	// Count the transaction
	countedTxs.push(txhash, accumulatedAmount)

	return allow()
}

// amountFromRaw converts a 32-byte raw value to human-readable units by dividing by 10^decimals.
// Supports common decimals: 18 (ETH/wei), 6 (USDC on most chains), 8, etc.
func amountFromRaw(value [32]byte, decimals uint8) uint64 {
	divisor := uint64(1)
	for i := uint8(0); i < decimals; i++ {
		divisor *= 10
	}
	const maxResult = (math.MaxUint64 - 255) / 256 // overflow threshold for result*256 + quotient
	var remHi, remLo uint64                        // 128-bit remainder
	var result uint64
	for _, b := range value {
		// val = remainder*256 + b (128-bit)
		valHi := remHi<<8 | remLo>>56
		valLo := remLo<<8 | uint64(b)
		quotient, remainder := bits.Div64(valHi, valLo, divisor)
		remHi, remLo = 0, remainder // remainder < divisor, fits in 64 bits

		if result >= maxResult {
			return math.MaxUint64 // overflow, value too large
		}
		result = result*256 + quotient
	}
	return result
}

func allow() (bool, string, error) {
	return false, "", nil
}

func block(reason string) (bool, string, error) {
	return true, reason, nil
}

func warn(reason string) {
	log.Warn("Exceed warning threshold", "plugin", "plugintransfer", "reason", reason)
}

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

// Ok, if the amout is same as the threshold
type ThresholdConfig struct {
	WarningCountThreshold  uint   `json:"warning_tx_count_threshold"`
	BlockCountThreshold    uint   `json:"block_count_threshold"`
	WarningAmountThreshold uint64 `json:"warning_amount_threshold"`
	BlockAmountThreshold   uint64 `json:"block_amount_threshold"`
}

type ConfigCache struct {
	Config    PluginConfig
	updatedAt time.Time
	ttl       time.Duration
	client    http.Client
}

func (c *ConfigCache) isExpired() bool {
	return c.updatedAt.Before(time.Now().Add(-c.ttl))
}

func (c *ConfigCache) isEmptyConfig() bool {
	return c.Config.MeasurementWindow == 0
}

func (c *ConfigCache) update() error {
	oldVersion := c.Config.Version
	resp, err := c.client.Get(configURL)
	if err != nil {
		return fmt.Errorf("failed to download config: url: %s, err: %w", configURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("download error: status %d, url: %s", resp.StatusCode, configURL)
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&c.Config); err != nil {
		return fmt.Errorf("failed to decode config: %w, url: %s", err, configURL)
	}

	// Update the last updated time
	c.updatedAt = time.Now()

	//
	if oldVersion != c.Config.Version || countedTxs == nil {
		newCap := c.Config.Threshold.BlockCountThreshold
		if countedTxs == nil {
			countedTxs = newCache(newCap)
		} else if newCap > countedTxs.cap {
			countedTxs.expandCap(newCap)
		}
	}

	return nil
}

func (c *ConfigCache) isWhitelisted(address common.Address) bool {
	return c.Config.Whitelists[address]
}

func (c *ConfigCache) isTargetERC20(address common.Address) (*TargetERC20Config, bool) {
	for _, target := range c.Config.TargetERC20s {
		if target.Address == address {
			return &target, true
		}
	}
	return nil, false
}

func (c *ConfigCache) toYen(target *TargetERC20Config, value [32]byte) uint64 {
	if target == nil { // native token
		return uint64(float64(amountFromRaw(value, 18)) * c.Config.NativeToken.ToYenRate)
	}
	return uint64(float64(amountFromRaw(value, target.Decimals)) * target.ToYenRate)
}

func isOverThreshold(config *PluginConfig, lrucache *lrucache, amount uint64) (blocks bool, reason string) {
	// Check warning count threshold
	if b, r := checkCountThreshold(lrucache, config.Threshold.WarningCountThreshold, config.MeasurementWindow, "warning"); b {
		warn(r)

		// Continue to check block count threshold
		if b, r := checkCountThreshold(lrucache, config.Threshold.BlockCountThreshold, config.MeasurementWindow, "block"); b {
			blocks = true
			reason = r
			// return -> continue checking amount threshold
		}
	}

	// Check warning amount threadhold
	window := config.MeasurementWindow
	if b, r, _ := checkAmountThreshold(lrucache, config.Threshold.WarningAmountThreshold, window, amount); b {
		warn(r)

		// Continue to check block amount threshold
		if b, r, _ := checkAmountThreshold(lrucache, config.Threshold.BlockAmountThreshold, window, amount); b {
			blocks = true
			if reason != "" {
				reason = fmt.Sprintf("%s, %s", reason, r)
			} else {
				reason = r
			}
			return true, reason
		}
	}

	return
}

func checkCountThreshold(c *lrucache, threshold uint, window time.Duration, level string) (blocks bool, reason string) {
	n := c.len()
	windowStart := time.Now().Add(-window)

	if n >= threshold {
		meta := c.get(n - threshold)
		if meta != nil && !meta.createdAt.Before(windowStart) {
			blocks = true
			reason = fmt.Sprintf("over %s count threshold: %d", level, threshold)
			return
		}
	}

	return
}

func computeTotal(c *lrucache, target *metadata, currentAmount uint64) uint64 {
	newestMeta := c.getNewest()
	accumulatedAmountIncludeCurrent := newestMeta.accumulatedAmount + newestMeta.amount + currentAmount
	isOddOverflow := newestMeta.isOddOverflow
	if math.MaxUint64-newestMeta.accumulatedAmount < newestMeta.amount+currentAmount {
		isOddOverflow = !newestMeta.isOddOverflow // overflow, flip the overflow flag
	}
	if target.isOddOverflow == isOddOverflow {
		return accumulatedAmountIncludeCurrent - target.accumulatedAmount
	}
	return math.MaxUint64 - target.accumulatedAmount + accumulatedAmountIncludeCurrent + 1
}

func checkAmountThreshold(c *lrucache, threshold uint64, window time.Duration, currentAmount uint64) (block bool, reason string, midindex uint /* <- midindex is used for testing */) {
	windowStart := time.Now().Add(-window)

	// Check if the current tx exceeds the threshold
	if threshold < currentAmount {
		return true, fmt.Sprintf("over block amount threshold: %d (single tx)", threshold), 0
	}

	// Skip if the cache is empty
	n := c.len()
	if n == 0 {
		return
	}

	// Check if the sum of all doesn't exceed the threshold
	if computeTotal(c, c.getOldest(), currentAmount) <= threshold {
		return
	}

	// Binary search to find the first index where the sum of all exceeds the threshold within the measurement window
	low, high := uint(0), n
	for low <= high {
		midindex = (low + high) / 2
		if n <= midindex { // mid is out of range
			break
		}
		midMeta := c.get(midindex)
		sum := computeTotal(c, midMeta, currentAmount)
		if sum <= threshold { // not exceed threshold
			if midMeta.createdAt.Before(windowStart) {
				// Window elapsed, no need to check further
				return
			}
			high = midindex - 1 // move to the leff (go older items)
		} else { // exceed threshold
			if midMeta.createdAt.After(windowStart) {
				// Within window, found!
				return true, fmt.Sprintf("over block amount threshold: %d, window sum: %d, current tx amount: %d", threshold, sum, currentAmount), midindex
			}
			low = midindex + 1 // move to the right (go newer items)
		}
	}

	return
}

type metadata struct {
	amount            uint64
	createdAt         time.Time
	accumulatedAmount uint64 // sum of previous metadata's amount, don't include the current amount
	isOddOverflow     bool   // true if the times of overflow is odd(e.g. 1, 3, 5, ...)
}

type lrucache struct {
	head  uint // index of the next slot to be used
	cap   uint // capacity of the lrucache
	items []common.Hash
	txMap map[common.Hash]*metadata
}

func newCache(cap uint) *lrucache {
	c := lrucache{
		head:  0,
		cap:   cap,
		items: make([]common.Hash, cap),
		txMap: make(map[common.Hash]*metadata, cap),
	}
	return &c
}

func (c *lrucache) push(txhash common.Hash, amount uint64) {
	var meta *metadata
	evicting := c.items[c.head] != emptyHash
	if evicting {
		meta = c.txMap[c.items[c.head]] // reuse the will-be-evicted item
	} else {
		meta = new(metadata)
	}
	meta.amount = amount
	meta.createdAt = time.Now()

	// Set accumulated amount and overflow flag
	prevSlot := (c.head + c.cap - 1) % c.cap
	if c.len() > 0 && c.txMap[c.items[prevSlot]] != nil {
		prevRecord := c.txMap[c.items[prevSlot]]
		meta.accumulatedAmount = prevRecord.accumulatedAmount + prevRecord.amount
		if math.MaxUint64-prevRecord.accumulatedAmount < prevRecord.amount {
			// overflow, flip the overflow flag
			meta.isOddOverflow = !prevRecord.isOddOverflow
		} else {
			meta.isOddOverflow = prevRecord.isOddOverflow
		}
	} else {
		meta.accumulatedAmount = 0 // first record: no previous
	}

	if evicting {
		delete(c.txMap, c.items[c.head]) // evict the oldest item
	}
	c.txMap[txhash] = meta
	c.items[c.head] = txhash
	c.head = (c.head + 1) % c.cap
}

func (c *lrucache) contains(txhash common.Hash) bool {
	_, ok := c.txMap[txhash]
	return ok
}

func (c *lrucache) get(index uint) *metadata {
	n := c.len()
	if index >= n {
		return nil
	}
	// When not full: data at slots 0..head-1 (oldest=0). When full: oldest at head.
	physicalSlot := c.physicalSlot(index)
	return c.txMap[c.items[physicalSlot]]
}

// physicalSlot returns the items slice index for logical index i (0=oldest).
func (c *lrucache) physicalSlot(logicalIndex uint) uint {
	if c.len() < c.cap {
		return logicalIndex // not wrapped: data is linear at 0..len-1
	}
	return (c.head + logicalIndex) % c.cap
}

func (c *lrucache) getOldest() *metadata {
	return c.get(0)
}

func (c *lrucache) getNewest() *metadata {
	n := c.len()
	if n == 0 {
		return nil
	}
	return c.get(n - 1)
}

func (c *lrucache) len() uint {
	if c.items[c.head] == emptyHash {
		return c.head
	}
	return c.cap
}

// expandCap grows the cache to newCap, keeping all existing entries and their LRU order.
func (c *lrucache) expandCap(newCap uint) {
	if newCap <= c.cap {
		return // No-op if newCap <= current cap
	}
	currentLen := c.len()
	newItems := make([]common.Hash, newCap)
	// Copy in LRU order: oldest at 0, newest at currentLen-1
	for i := uint(0); i < currentLen; i++ {
		newItems[i] = c.items[c.physicalSlot(i)]
	}
	c.items = newItems
	c.cap = newCap
	c.head = currentLen
}
