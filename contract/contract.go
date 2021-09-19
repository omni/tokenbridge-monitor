package contract

import (
	"amb-monitor/entity"
	"amb-monitor/ethclient"
	"fmt"

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

func (c *Contract) ParseLog(log *entity.Log) (string, map[string]interface{}, error) {
	if log.Topic0 == nil {
		return "", nil, fmt.Errorf("cannot process event without topics")
	}
	event, err := c.abi.EventByID(*log.Topic0)
	if err != nil {
		return "", nil, nil
	}
	m := make(map[string]interface{})
	err = event.Inputs.UnpackIntoMap(m, log.Data)
	if err != nil {
		return "", nil, err
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
	err = abi.ParseTopicsIntoMap(m, Indexed(event.Inputs), topics)
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
