package monitor

import "github.com/ethereum/go-ethereum/common"

func uintPtr(v uint) *uint {
	return &v
}

func hashPtr(v common.Hash) *common.Hash {
	return &v
}
