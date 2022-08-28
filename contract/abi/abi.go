package abi

import (
	"errors"
	"fmt"
	"strings"

	gethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/omni/tokenbridge-monitor/entity"
)

type ABI struct {
	gethabi.ABI
}

type Event struct {
	gethabi.Event
}

var ErrInvalidEvent = errors.New("invalid event")

func MustReadABI(rawJSON string) ABI {
	res, err := gethabi.JSON(strings.NewReader(rawJSON))
	if err != nil {
		panic(err)
	}
	return ABI{res}
}

func (abi *ABI) AllEvents() map[string]bool {
	events := make(map[string]bool, len(abi.Events))
	for _, event := range abi.Events {
		events[event.String()] = true
	}
	return events
}

func (abi *ABI) FindMatchingEventABI(topics []common.Hash) *Event {
	for _, e := range abi.Events {
		if e.ID == topics[0] {
			indexed := Indexed(e.Inputs)
			if len(indexed) == len(topics)-1 {
				return &Event{e}
			}
		}
	}
	return nil
}

func (abi *ABI) ParseLog(log *entity.Log) (string, map[string]interface{}, error) {
	topics := log.Topics()
	if len(topics) == 0 {
		return "", nil, fmt.Errorf("cannot process event without topics: %w", ErrInvalidEvent)
	}
	event := abi.FindMatchingEventABI(topics)
	if event == nil {
		return "", nil, nil
	}

	res, err := event.DecodeLogData(topics, log.Data)
	if err != nil {
		return "", nil, fmt.Errorf("can't decode event log data: %w", err)
	}
	return event.String(), res, nil
}

func (event *Event) DecodeLogData(topics []common.Hash, data []byte) (map[string]interface{}, error) {
	indexed := Indexed(event.Inputs)
	values := make(map[string]interface{})
	if len(indexed) < len(event.Inputs) {
		if err := event.Inputs.UnpackIntoMap(values, data); err != nil {
			return nil, fmt.Errorf("can't unpack data: %w", err)
		}
	}
	if err := gethabi.ParseTopicsIntoMap(values, indexed, topics[1:]); err != nil {
		return nil, fmt.Errorf("can't unpack topics: %w", err)
	}
	return values, nil
}

func Indexed(args gethabi.Arguments) gethabi.Arguments {
	var indexed gethabi.Arguments
	for _, arg := range args {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	return indexed
}
