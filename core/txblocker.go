package core

import (
	"fmt"
	"plugin"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type TxBlocker struct {
	plugin *plugin.Plugin
}

func NewTxBlocker(path string) (b *TxBlocker, err error) {
	b = &TxBlocker{}
	b.plugin, err = plugin.Open(path)
	return
}

func (b *TxBlocker) BlockTransaction(msg *Message, logs []*types.Log) (isBlocked bool, reason string, err error) {
	var (
		from, to   common.Address
		value      [32]byte
		copiedLogs = make([]types.Log, len(logs))
	)
	from = msg.From
	copy(to[:], msg.To[:])
	copy(value[:], msg.Value.Bytes())
	for i, log := range logs {
		copiedLogs[i].Topics = make([]common.Hash, len(log.Topics))
		for j, topic := range log.Topics {
			copy(copiedLogs[i].Topics[j][:], topic[:])
		}
	}

	// 2. Lookup the function symbol
	f, err := b.plugin.Lookup("BlockTransaction")
	if err != nil {
		panic(err)
	}

	// 3. Cast it to the correct function signature
	process := f.(func(common.Address, common.Address, [32]byte, []types.Log) (bool, string, error))

	// 4. Call the plugin function with arguments
	isBlocked, reason, err = process(from, to, value, copiedLogs)
	fmt.Println("BlockTransaction result", isBlocked, "reason", reason, "err", err)
	return false, "", nil
}
