package abi_test

import (
	"bytes"
	_ "embed"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/poanetwork/tokenbridge-monitor/contract/abi"
	"github.com/poanetwork/tokenbridge-monitor/entity"
)

//go:embed test_abi.json
var testJSONABI string

var (
	transferTopic         = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
	testEventTopic        = crypto.Keccak256Hash([]byte("TestEvent(uint256,uint256)"))
	testIndexedEventTopic = crypto.Keccak256Hash([]byte("TestIndexedEvent(uint256,uint256)"))
	aliceAddr             = common.HexToAddress("0x01")
	alice                 = aliceAddr.Hash()
	bobAddr               = common.HexToAddress("0x02")
	bob                   = bobAddr.Hash()
)

func TestABI_AllEvents(t *testing.T) {
	t.Parallel()

	testABI := abi.MustReadABI(testJSONABI)

	allEvents := testABI.AllEvents()
	require.Equal(t, map[string]bool{
		"event Transfer(address indexed sender, address indexed receiver, uint256 value)": true,
		"event TestEvent(uint256 a, uint256 b)":                                           true,
		"event TestIndexedEvent(uint256 indexed a, uint256 indexed b)":                    true,
	}, allEvents)
}

func TestABI_FindMatchingEventABI(t *testing.T) {
	t.Parallel()

	testABI := abi.MustReadABI(testJSONABI)

	event := testABI.FindMatchingEventABI([]common.Hash{transferTopic, alice, bob})
	require.NotNil(t, event)
	require.Equal(t, "Transfer", event.Name)
	event = testABI.FindMatchingEventABI([]common.Hash{transferTopic, alice})
	require.Nil(t, event)
	event = testABI.FindMatchingEventABI([]common.Hash{transferTopic, alice, bob, alice})
	require.Nil(t, event)
	event = testABI.FindMatchingEventABI([]common.Hash{testEventTopic})
	require.NotNil(t, event)
	require.Equal(t, "TestEvent", event.Name)
}

func TestABI_ParseLog(t *testing.T) {
	t.Parallel()

	testABI := abi.MustReadABI(testJSONABI)

	value := big.NewInt(100)
	valueHash := common.BigToHash(value)
	logData := valueHash.Bytes()

	t.Run("should parse valid transfer event", func(t *testing.T) {
		t.Parallel()
		log := &entity.Log{Topic0: &transferTopic, Topic1: &alice, Topic2: &bob, Data: logData}
		event, data, err := testABI.ParseLog(log)
		require.NoError(t, err)
		require.Equal(t, "event Transfer(address indexed sender, address indexed receiver, uint256 value)", event)
		require.Equal(t, map[string]interface{}{
			"sender":   aliceAddr,
			"receiver": bobAddr,
			"value":    value,
		}, data)
	})

	t.Run("should not parse anonymous event", func(t *testing.T) {
		t.Parallel()
		log := &entity.Log{Data: logData}
		event, data, err := testABI.ParseLog(log)
		require.ErrorIs(t, err, abi.ErrInvalidEvent)
		require.Empty(t, event)
		require.Empty(t, data)
	})

	t.Run("should skip unknown event", func(t *testing.T) {
		t.Parallel()
		log := &entity.Log{Topic0: &transferTopic, Data: logData}
		event, data, err := testABI.ParseLog(log)
		require.NoError(t, err)
		require.Empty(t, event)
		require.Empty(t, data)
	})

	t.Run("should decode event without indexed fields", func(t *testing.T) {
		t.Parallel()
		log := &entity.Log{Topic0: &testEventTopic, Data: bytes.Repeat(logData, 2)}
		event, data, err := testABI.ParseLog(log)
		require.NoError(t, err)
		require.Equal(t, "event TestEvent(uint256 a, uint256 b)", event)
		require.Equal(t, map[string]interface{}{
			"a": value,
			"b": value,
		}, data)
	})

	t.Run("should decode event with only indexed fields", func(t *testing.T) {
		t.Parallel()
		log := &entity.Log{Topic0: &testIndexedEventTopic, Topic1: &valueHash, Topic2: &valueHash}
		event, data, err := testABI.ParseLog(log)
		require.NoError(t, err)
		require.Equal(t, "event TestIndexedEvent(uint256 indexed a, uint256 indexed b)", event)
		require.Equal(t, map[string]interface{}{
			"a": value,
			"b": value,
		}, data)
	})

	t.Run("should fail to decode event with incompatible ABI", func(t *testing.T) {
		t.Parallel()
		log := &entity.Log{Topic0: &testEventTopic, Data: logData}
		event, data, err := testABI.ParseLog(log)
		require.Error(t, err)
		require.Contains(t, err.Error(), "length insufficient")
		require.Empty(t, event)
		require.Empty(t, data)
	})
}
