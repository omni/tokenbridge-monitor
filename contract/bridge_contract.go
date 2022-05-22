package contract

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/contract/abi"
	"github.com/poanetwork/tokenbridge-monitor/ethclient"
)

type BridgeContract struct {
	*Contract
}

func NewBridgeContract(client ethclient.Client, addr common.Address, mode config.BridgeMode) *BridgeContract {
	var contract *Contract
	switch mode {
	case config.BridgeModeArbitraryMessage:
		contract = NewContract(client, addr, abi.ArbitraryMessageABI)
	case config.BridgeModeErcToNative:
		contract = NewContract(client, addr, abi.ErcToNativeABI)
	default:
		contract = NewContract(client, addr, abi.ArbitraryMessageABI)
	}
	return &BridgeContract{contract}
}

func (c *BridgeContract) ValidatorContractAddress(ctx context.Context) (common.Address, error) {
	res, err := c.Call(ctx, "validatorContract")
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot obtain validator contract address: %w", err)
	}
	return common.BytesToAddress(res), nil
}
