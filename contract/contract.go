package contract

import (
	"amb-monitor/entity"
	"amb-monitor/ethclient"
	"bytes"
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type Contract struct {
	Address common.Address
	client  *ethclient.Client
	abi     abi.ABI
}

func NewContract(client *ethclient.Client, addr common.Address, abi abi.ABI) *Contract {
	return &Contract{addr, client, abi}
}

func (c *Contract) HasEvent(event string) bool {
	_, ok := c.abi.Events[event]
	return ok
}

func (c *Contract) ValidatorContractAddress(ctx context.Context) (common.Address, error) {
	data, err := c.abi.Pack("validatorContract")
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot encode abi calldata: %w", err)
	}
	res, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.Address,
		Data: data,
	})
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot call validatorContract(): %w", err)
	}
	return common.BytesToAddress(res), nil
}

func (c *Contract) ParseLog(log *entity.Log) (string, map[string]interface{}, error) {
	if log.Topic0 == nil {
		return "", nil, fmt.Errorf("cannot process event without topics")
	}
	topics := make([]common.Hash, 0, 3)
	if log.Topic1 != nil {
		topics = append(topics, *log.Topic1)
		if log.Topic2 != nil {
			topics = append(topics, *log.Topic2)
			if log.Topic3 != nil {
				topics = append(topics, *log.Topic3)
			}
		}
	}
	var event *abi.Event
	var indexed abi.Arguments
	for _, e := range c.abi.Events {
		if bytes.Equal(e.ID.Bytes(), log.Topic0.Bytes()) {
			indexed = Indexed(e.Inputs)
			if len(indexed) == len(topics) {
				event = &e
				break
			}
		}
	}
	if event == nil {
		return "", nil, nil
	}
	m := make(map[string]interface{})
	if len(indexed) < len(event.Inputs) {
		if err := event.Inputs.UnpackIntoMap(m, log.Data); err != nil {
			return "", nil, fmt.Errorf("can't unpack data: %w", err)
		}
	}
	if err := abi.ParseTopicsIntoMap(m, indexed, topics); err != nil {
		return "", nil, fmt.Errorf("can't unpack topics: %w", err)
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
