package contract

import (
	"amb-monitor/ethclient"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Contract struct {
	Address common.Address
	client  *ethclient.Client
	abi     abi.ABI
}

func NewContract(client *ethclient.Client, addr string, abi abi.ABI) *Contract {
	return &Contract{common.HexToAddress(addr), client, abi}
}

func (c *Contract) HasEvent(event string) bool {
	_, ok := c.abi.Events[event]
	return ok
}

func (c *Contract) Call(method string, args ...interface{}) ([]interface{}, error) {
	data, err := c.abi.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	ctx, cancel := c.client.GetCtx()
	defer cancel()
	msg := ethereum.CallMsg{
		To:   &c.Address,
		Data: data,
	}
	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, err
	}
	return c.abi.Unpack(method, result)
}

func (c *Contract) ParseLog(log *types.Log) (string, map[string]interface{}, error) {
	if len(log.Topics) == 0 {
		return "", nil, fmt.Errorf("cannot process event without topics")
	}
	event, err := c.abi.EventByID(log.Topics[0])
	if err != nil {
		return "", nil, nil
	}
	m := make(map[string]interface{})
	err = event.Inputs.UnpackIntoMap(m, log.Data)
	if err != nil {
		return "", nil, err
	}
	err = abi.ParseTopicsIntoMap(m, Indexed(event.Inputs), log.Topics[1:])
	if err != nil {
		return "", nil, err
	}
	return event.Name, m, nil
}

func Indexed(args abi.Arguments) abi.Arguments {
	var indexed abi.Arguments
	for _, arg := range args {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	return indexed
}
