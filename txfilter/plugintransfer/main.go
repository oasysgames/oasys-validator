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

// Call before loading a new plugin instance.
// Clear drops references to runtime state so GC can reclaim memory
func Clear() error {
	countedTxs = nil
	configCache.Config = PluginConfig{}
	configCache.updatedAt = time.Time{}
	return nil
}

func FilterTransaction(txhash common.Hash, from, to common.Address, value [32]byte, logs []types.Log) (isBlocked bool, reason string, err error) {
	// Initialize or update it if it's expired
	if configCache.isExpired() {
		if err := configCache.update(); err != nil {
			if configCache.isEmptyConfig() {
				// No block, just return error to not block chain execution but notify the error to the caller
				return false, "", fmt.Errorf("failed to fetch config: %w", err)
			}
			// Just log the error, keep using the old config
			log.Error("failed to fetch config", "error", err, "url", configURL)
		}
	}

	// Exit if the config is empty
	if countedTxs == nil || configCache.isEmptyConfig() {
		// No block, but notify the error to the caller
		return false, "", fmt.Errorf("config is empty")
	}

	// Skip if the plugin is disabled, or whitelisted, or already counted
	if configCache.Config.Disabled || configCache.isWhitelisted(from) || countedTxs.contains(txhash) {
		return allow()
	}

	// initialize accumulated amount by native token amount
	accumulatedAmount := configCache.toYen(nil, value)

	for i := range logs {
		// Skip if the log is not a transfer event
		if len(logs[i].Topics) != 3 || logs[i].Topics[0] != transferEventTopic {
			continue
		}
		// Skip if the log is not a target ERC20
		target, ok := configCache.isTargetERC20(logs[i].Address)
		if !ok {
			continue
		}
		// The amount is stored in `log.Data` as a 32-byte value.
		// sanity check: this should not occur in a standard ERC20 transfer
		if len(logs[i].Data) != 32 {
			continue
		}
		// Add accumulated amount by ERC20 token amount
		var amount [32]byte
		copy(amount[:], logs[i].Data)
		accumulatedAmount += configCache.toYen(target, amount)
	}

	// Skip if the accumulated amount is zero or smaller than 1 yen
	if accumulatedAmount == 0 {
		return allow()
	}

	// Check the count and amount thresholds
	now := time.Now().Unix()
	if blocks, reason := isOverThreshold(&configCache.Config, accumulatedAmount, now); blocks {
		return block(reason)
	}

	// Count the transaction
	countedTxs.push(txhash, accumulatedAmount, now)

	return allow()
}

func allow() (bool, string, error) {
	return false, "", nil
}

func block(reason string) (bool, string, error) {
	return true, reason, nil
}

func logWarn(reason string) {
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
	return c.Config.MeasurementWindow == 0 || c.Config.Threshold.BlockCountThreshold == 0 || c.Config.Threshold.BlockAmountThreshold == 0
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

	// Initialize or expand the countedTxs cache
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
	for i := range c.Config.TargetERC20s {
		if c.Config.TargetERC20s[i].Address == address {
			return &c.Config.TargetERC20s[i], true
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

func amountFromRaw(value [32]byte, decimals uint8) uint64 {
	divisor := uint64(1)
	for i := uint8(0); i < decimals; i++ {
		divisor *= 10
	}
	const maxResult = (math.MaxUint64 - 255) / 256 // overflow threshold for result*256 + quotient
	var remHi, remLo uint64                        // 128-bit remainder
	var result uint64
	for i := range value {
		// val = remainder*256 + b (128-bit)
		valHi := remHi<<8 | remLo>>56
		valLo := remLo<<8 | uint64(value[i])
		quotient, remainder := bits.Div64(valHi, valLo, divisor)
		remHi, remLo = 0, remainder // remainder < divisor, fits in 64 bits

		if result >= maxResult {
			return math.MaxUint64 // overflow, value too large
		}
		result = result*256 + quotient
	}
	return result
}

func isOverThreshold(config *PluginConfig, amount uint64, now int64) (blocks bool, reason string) {
	// Check warning count threshold
	startTime := now - int64(config.MeasurementWindow/time.Second)
	if b, r := checkCountThreshold(config.Threshold.WarningCountThreshold, startTime); b {
		logWarn(r)

		// Continue to check block count threshold
		if b, r := checkCountThreshold(config.Threshold.BlockCountThreshold, startTime); b {
			blocks = true
			reason = r
			// return -> continue checking amount threshold
		}
	}

	// Check warning amount threadhold
	if b, r, _ := checkAmountThreshold(config.Threshold.WarningAmountThreshold, startTime, amount); b {
		logWarn(r)

		// Continue to check block amount threshold
		if b, r, _ := checkAmountThreshold(config.Threshold.BlockAmountThreshold, startTime, amount); b {
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

func checkCountThreshold(threshold uint, startTime int64) (blocks bool, reason string) {
	c := countedTxs
	n := c.len()
	if n >= threshold {
		meta, ok := c.get(n - threshold)
		if ok && meta.createdAt >= startTime {
			blocks = true
			reason = fmt.Sprintf("over count threshold: %d", threshold)
			return
		}
	}

	return
}

func checkAmountThreshold(threshold uint64, startTime int64, currentAmount uint64) (block bool, reason string, midindex uint /* <- midindex is used for testing */) {
	c := countedTxs

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
	oldestMeta, _ := c.getOldest()
	if computeTotal(oldestMeta, currentAmount) <= threshold {
		return
	}

	// Binary search to find the first index where the sum of all exceeds the threshold within the measurement window
	low, high := uint(0), n
	for low <= high {
		midindex = (low + high) / 2
		midMeta, ok := c.get(midindex)
		if !ok {
			break // mid is out of range
		}
		sum := computeTotal(midMeta, currentAmount)
		if sum <= threshold { // not exceed threshold
			if midMeta.createdAt < startTime {
				// Window elapsed, no need to check further
				return
			}
			high = midindex - 1 // move to the leff (go older items)
		} else { // exceed threshold
			if midMeta.createdAt >= startTime {
				// Within window, found!
				return true, fmt.Sprintf("over block amount threshold: %d, window sum: %d, current tx amount: %d", threshold, sum, currentAmount), midindex
			}
			low = midindex + 1 // move to the right (go newer items)
		}
	}

	return
}

func computeTotal(target metadata, currentAmount uint64) uint64 {
	c := countedTxs
	newestMeta, ok := c.getNewest()
	if !ok {
		return currentAmount
	}
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

type metadata struct {
	amount            uint64
	createdAt         int64  // Unix timestamp
	accumulatedAmount uint64 // sum of previous metadata's amount, don't include the current amount
	isOddOverflow     bool   // true if the times of overflow is odd(e.g. 1, 3, 5, ...)
}

type lrucache struct {
	head  uint // index of the next slot to be used
	cap   uint // capacity of the lrucache
	items []common.Hash
	txMap map[common.Hash]metadata
}

func newCache(cap uint) *lrucache {
	c := lrucache{
		head:  0,
		cap:   cap,
		items: make([]common.Hash, cap),
		txMap: make(map[common.Hash]metadata, cap),
	}
	return &c
}

func (c *lrucache) push(txhash common.Hash, amount uint64, now int64) {
	meta, evicting := c.txMap[c.items[c.head]] // reuse the will-be-evicted item
	if !evicting {
		meta = metadata{} // create a new metadata, items array is not full yet
	}
	meta.amount = amount
	meta.createdAt = now

	// Set accumulated amount and overflow flag
	prevSlot := (c.head + c.cap - 1) % c.cap
	prevRecord, ok := c.txMap[c.items[prevSlot]]
	if c.len() > 0 && ok {
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

func (c *lrucache) get(index uint) (metadata, bool) {
	n := c.len()
	if index >= n {
		return metadata{}, false
	}
	slot := c.physicalSlot(index)
	meta, ok := c.txMap[c.items[slot]]
	return meta, ok
}

// physicalSlot returns the items slice index for logical index i (0=oldest).
func (c *lrucache) physicalSlot(logicalIndex uint) uint {
	if c.len() < c.cap {
		return logicalIndex // not wrapped: data is linear at 0..len-1
	}
	return (c.head + logicalIndex) % c.cap
}

func (c *lrucache) getOldest() (metadata, bool) {
	return c.get(0)
}

func (c *lrucache) getNewest() (metadata, bool) {
	n := c.len()
	if n == 0 {
		return metadata{}, false
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
