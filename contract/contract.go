package contract

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"

	"github.com/poanetwork/tokenbridge-monitor/contract/abi"
	"github.com/poanetwork/tokenbridge-monitor/ethclient"
)

type Contract struct {
	client  ethclient.Client
	Address common.Address
	ABI     abi.ABI
}

func NewContract(client ethclient.Client, addr common.Address, abi abi.ABI) *Contract {
	return &Contract{client, addr, abi}
}

func (c *Contract) Call(ctx context.Context, method string, args ...interface{}) ([]byte, error) {
	data, err := c.ABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("cannot encode abi calldata: %w", err)
	}
	res, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.Address,
		Data: data,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot call %s(...): %w", method, err)
	}
	return res, nil
}
