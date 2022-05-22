package contract

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/poanetwork/tokenbridge-monitor/entity"
	"github.com/poanetwork/tokenbridge-monitor/ethclient"
)

type Contract struct {
	address common.Address
	client  ethclient.Client
	abi     abi.ABI
}

func NewContract(client ethclient.Client, addr common.Address, abi abi.ABI) *Contract {
	return &Contract{addr, client, abi}
}

func (c *Contract) AllEvents() map[string]bool {
	events := make(map[string]bool, len(c.abi.Events))
	for _, event := range c.abi.Events {
		events[event.String()] = true
	}
	return events
}

func (c *Contract) Call(ctx context.Context, method string, args ...interface{}) ([]byte, error) {
	data, err := c.abi.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("cannot encode abi calldata: %w", err)
	}
	res, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.address,
		Data: data,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot call %s(...): %w", method, err)
	}
	return res, nil
}

func (c *Contract) ParseLog(log *entity.Log) (string, map[string]interface{}, error) {
	return ParseLog(c.abi, log)
}
