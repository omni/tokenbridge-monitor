package contract

import (
	"context"
	"fmt"
	"tokenbridge-monitor/config"
	"tokenbridge-monitor/contract/abi"
	"tokenbridge-monitor/ethclient"

	"github.com/ethereum/go-ethereum/common"
)

type BridgeContract struct {
	*Contract
}

func NewBridgeContract(client ethclient.Client, addr common.Address, mode config.BridgeMode) *BridgeContract {
	var contract *Contract
	switch mode {
	case config.BridgeModeArbitraryMessage:
		contract = NewContract(client, addr, abi.AMB)
	case config.BridgeModeErcToNative:
		contract = NewContract(client, addr, abi.ERC_TO_NATIVE)
	default:
		contract = NewContract(client, addr, abi.AMB)
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
