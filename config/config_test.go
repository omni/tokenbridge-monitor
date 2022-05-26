package config_test

import (
	"math"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/poanetwork/tokenbridge-monitor/config"
)

const testCfg = `
chains:
  mainnet:
    rpc:
      host: https://mainnet.infura.io/v3/${INFURA_PROJECT_KEY}
      timeout: 30s
      rps: 10
    chain_id: 1
    block_time: 15s
    block_index_interval: 60s
  xdai:
    rpc:
      host: https://rpc.ankr.com/gnosis
      timeout: 20s
      rps: 10
    chain_id: 100
    block_time: 5s
    block_index_interval: 30s
    safe_logs_request: true
bridges:
  xdai:
    bridge_mode: ERC_TO_NATIVE
    home:
      chain: xdai
      address: 0x7301CFA0e1756B71869E93d4e4Dca5c7d0eb0AA6
      validator_contract_address: 0xB289f0e6fBDFf8EEE340498a56e1787B303F1B6D
      start_block: 756
      required_block_confirmations: 12
      max_block_range_size: 2000
    foreign:
      chain: mainnet
      address: 0x4aa42145Aa6Ebf72e164C9bBC74fbD3788045016
      validator_contract_address: 0xe1579dEbdD2DF16Ebdb9db8694391fa74EeA201E
      start_block: 6478411
      required_block_confirmations: 12
      erc_to_native_tokens:
        - address: 0x89d24A6b4CcB1B6fAA2625fE562bDD9a23260359
          end_block: 9884448
        - address: 0x6B175474E89094C44Da98b954EedeAC495271d0F
          start_block: 8928158
          blacklisted_senders:
            - 0x0000000000000000000000000000000000000000
            - 0x5d3a536E4D6DbD6114cc1Ead35777bAB948E3643
    alerts:
      unknown_erc_to_native_message_confirmation:
      unknown_erc_to_native_message_execution:
      stuck_erc_to_native_message_confirmation:
      last_validator_activity:
  xdai-amb:
    bridge_mode: AMB
    home:
      chain: xdai
      address: 0x75Df5AF045d91108662D8080fD1FEFAd6aA0bb59
      start_block: 7408640
      required_block_confirmations: 12
      max_block_range_size: 2000
      whitelisted_senders:
        - 0x73cA9C4e72fF109259cf7374F038faf950949C51
    foreign:
      chain: mainnet
      address: 0x4C36d2919e407f0Cc2Ee3c993ccF8ac26d9CE64e
      start_block: 9130277
      required_block_confirmations: 12
    alerts:
      unknown_message_confirmation:
      unknown_message_execution:
      stuck_message_confirmation:
        foreign_start_block: 12922477
      failed_message_execution:
        home_start_block: 19979926
        foreign_start_block: 13897393
postgres:
  user: test_user
  password: test_password
  host: test_host
  port: 5432
  database: test_db
log_level: info
presenter:
  host: 0.0.0.0:3333
`

//nolint:paralleltest
func TestReadConfigWithEnv(t *testing.T) {
	t.Setenv("INFURA_PROJECT_KEY", "12345678")
	cfg, err := config.ReadConfigWithEnv([]byte(testCfg))
	require.NoError(t, err)
	mainnetChainCfg := &config.ChainConfig{
		RPC: &config.RPCConfig{
			Host:    "https://mainnet.infura.io/v3/12345678",
			Timeout: 30 * time.Second,
			RPS:     10,
		},
		ChainID:            "1",
		BlockTime:          15 * time.Second,
		BlockIndexInterval: 60 * time.Second,
		SafeLogsRequest:    false,
	}
	xdaiChainCfg := &config.ChainConfig{
		RPC: &config.RPCConfig{
			Host:    "https://rpc.ankr.com/gnosis",
			Timeout: 20 * time.Second,
			RPS:     10,
		},
		ChainID:            "100",
		BlockTime:          5 * time.Second,
		BlockIndexInterval: 30 * time.Second,
		SafeLogsRequest:    true,
	}
	require.Equal(t, &config.Config{
		Chains: map[string]*config.ChainConfig{
			"mainnet": mainnetChainCfg,
			"xdai":    xdaiChainCfg,
		},
		Bridges: map[string]*config.BridgeConfig{
			"xdai": {
				ID:         "xdai",
				BridgeMode: config.BridgeModeErcToNative,
				Home: &config.BridgeSideConfig{
					ChainName:                "xdai",
					Chain:                    xdaiChainCfg,
					Address:                  common.HexToAddress("0x7301CFA0e1756B71869E93d4e4Dca5c7d0eb0AA6"),
					ValidatorContractAddress: common.HexToAddress("0xB289f0e6fBDFf8EEE340498a56e1787B303F1B6D"),
					StartBlock:               756,
					BlockConfirmations:       12,
					MaxBlockRangeSize:        2000,
				},
				Foreign: &config.BridgeSideConfig{
					ChainName:                "mainnet",
					Chain:                    mainnetChainCfg,
					Address:                  common.HexToAddress("0x4aa42145Aa6Ebf72e164C9bBC74fbD3788045016"),
					ValidatorContractAddress: common.HexToAddress("0xe1579dEbdD2DF16Ebdb9db8694391fa74EeA201E"),
					StartBlock:               6478411,
					BlockConfirmations:       12,
					MaxBlockRangeSize:        1000,
					ErcToNativeTokens: []config.TokenConfig{
						{
							Address:    common.HexToAddress("0x89d24A6b4CcB1B6fAA2625fE562bDD9a23260359"),
							StartBlock: 6478411,
							EndBlock:   9884448,
						},
						{
							Address:    common.HexToAddress("0x6B175474E89094C44Da98b954EedeAC495271d0F"),
							StartBlock: 8928158,
							EndBlock:   math.MaxUint32,
							BlacklistedSenders: []common.Address{
								common.HexToAddress("0x0000000000000000000000000000000000000000"),
								common.HexToAddress("0x5d3a536E4D6DbD6114cc1Ead35777bAB948E3643"),
							},
						},
					},
				},
				Alerts: map[string]*config.BridgeAlertConfig{
					"unknown_erc_to_native_message_confirmation": {756, 6478411},
					"unknown_erc_to_native_message_execution":    {756, 6478411},
					"stuck_erc_to_native_message_confirmation":   {756, 6478411},
					"last_validator_activity":                    {756, 6478411},
				},
			},
			"xdai-amb": {
				ID:         "xdai-amb",
				BridgeMode: config.BridgeModeArbitraryMessage,
				Home: &config.BridgeSideConfig{
					ChainName:          "xdai",
					Chain:              xdaiChainCfg,
					Address:            common.HexToAddress("0x75Df5AF045d91108662D8080fD1FEFAd6aA0bb59"),
					StartBlock:         7408640,
					BlockConfirmations: 12,
					MaxBlockRangeSize:  2000,
					WhitelistedSenders: []common.Address{
						common.HexToAddress("0x73cA9C4e72fF109259cf7374F038faf950949C51"),
					},
				},
				Foreign: &config.BridgeSideConfig{
					ChainName:          "mainnet",
					Chain:              mainnetChainCfg,
					Address:            common.HexToAddress("0x4C36d2919e407f0Cc2Ee3c993ccF8ac26d9CE64e"),
					StartBlock:         9130277,
					BlockConfirmations: 12,
					MaxBlockRangeSize:  1000,
				},
				Alerts: map[string]*config.BridgeAlertConfig{
					"unknown_message_confirmation": {7408640, 9130277},
					"unknown_message_execution":    {7408640, 9130277},
					"stuck_message_confirmation":   {7408640, 12922477},
					"failed_message_execution":     {19979926, 13897393},
				},
			},
		},
		DBConfig: &config.DBConfig{
			User:     "test_user",
			Password: "test_password",
			Host:     "test_host",
			Port:     5432,
			DB:       "test_db",
		},
		LogLevel:        logrus.InfoLevel,
		DisabledBridges: nil,
		EnabledBridges:  nil,
		Presenter: &config.PresenterConfig{
			Host: "0.0.0.0:3333",
		},
	}, cfg)
}

func TestBridgeSideConfig_ErcToNativeTokenAddresses(t *testing.T) {
	t.Parallel()
	cfg, err := config.ReadConfig([]byte(testCfg))
	require.NoError(t, err)
	tokenAddresses := cfg.Bridges["xdai"].Foreign.ErcToNativeTokenAddresses(7000000, 11000000)
	require.Equal(t, tokenAddresses, []common.Address{
		common.HexToAddress("0x89d24A6b4CcB1B6fAA2625fE562bDD9a23260359"),
		common.HexToAddress("0x6B175474E89094C44Da98b954EedeAC495271d0F"),
	})
	tokenAddresses = cfg.Bridges["xdai"].Foreign.ErcToNativeTokenAddresses(7000000, 8000000)
	require.Equal(t, tokenAddresses, []common.Address{
		common.HexToAddress("0x89d24A6b4CcB1B6fAA2625fE562bDD9a23260359"),
	})
	tokenAddresses = cfg.Bridges["xdai"].Foreign.ErcToNativeTokenAddresses(10000000, 11000000)
	require.Equal(t, tokenAddresses, []common.Address{
		common.HexToAddress("0x6B175474E89094C44Da98b954EedeAC495271d0F"),
	})
}

func TestBridgeSideConfig_ContractAddresses(t *testing.T) {
	t.Parallel()
	cfg, err := config.ReadConfig([]byte(testCfg))
	require.NoError(t, err)
	tokenAddresses := cfg.Bridges["xdai"].Foreign.ContractAddresses(7000000, 11000000)
	require.Equal(t, tokenAddresses, []common.Address{
		common.HexToAddress("0x89d24A6b4CcB1B6fAA2625fE562bDD9a23260359"),
		common.HexToAddress("0x6B175474E89094C44Da98b954EedeAC495271d0F"),
		common.HexToAddress("0x4aa42145Aa6Ebf72e164C9bBC74fbD3788045016"),
		common.HexToAddress("0xe1579dEbdD2DF16Ebdb9db8694391fa74EeA201E"),
	})
}
