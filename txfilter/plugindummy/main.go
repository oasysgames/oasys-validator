package main

import (
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

// version can be set at build time using -ldflags "-X main.version=1.0.0"
var version = "1.0.0"

// blockedByPlugin can be set at build time using -ldflags "-X main.blockedByPlugin=true"
var blockedByPlugin = "false"

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

func FilterTransaction(txhash common.Hash, from, to common.Address, value [32]byte, logs []types.Log) (isBlocked bool, reason string, err error) {
	if blockedByPlugin == "true" {
		// Given all the arguments, form a JSON string
		valueStr := hexutil.Encode(value[:])
		logsEntries := make([]LogEntry, len(logs))
		for i, log := range logs {
			logsEntries[i] = LogEntry{
				Address: log.Address.Hex(),
				Topics:  make([]string, len(log.Topics)),
				Data:    hexutil.Encode(log.Data[:]),
			}
			for j, topic := range log.Topics {
				logsEntries[i].Topics[j] = topic.Hex()
			}
		}
		reasonJSON := ReasonJSON{
			From:  from.Hex(),
			To:    to.Hex(),
			Value: valueStr,
			Logs:  logsEntries,
		}
		reasonBytes, err := json.Marshal(reasonJSON)
		if err != nil {
			return false, reason, errors.New("failed to marshal reason JSON")
		}
		reason = string(reasonBytes)
		return true, reason, errors.New("blocked by plugin")
	}
	return false, "", nil
}

func Version() string {
	return version
}
