package config

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var (
	ErrNonEmptyTokenList = errors.New("non-empty token address list")
	ErrEmptyTokenList    = errors.New("empty token address list")
	ErrInvalidConfig     = errors.New("invalid config")
)

type RPCConfig struct {
	Host    string        `yaml:"host" json:"-"` // hidden from public presenter endpoint
	Timeout time.Duration `yaml:"timeout"`
	RPS     float64       `yaml:"rps"`
}

type ChainConfig struct {
	RPC                *RPCConfig    `yaml:"rpc"`
	ChainID            string        `yaml:"chain_id"`
	BlockTime          time.Duration `yaml:"block_time"`
	BlockIndexInterval time.Duration `yaml:"block_index_interval"`
	SafeLogsRequest    bool          `yaml:"safe_logs_request"`
}

type TokenConfig struct {
	Address            common.Address   `yaml:"address"`
	StartBlock         uint             `yaml:"start_block"`
	EndBlock           uint             `yaml:"end_block"`
	BlacklistedSenders []common.Address `yaml:"blacklisted_senders"`
}

type BridgeSideConfig struct {
	ChainName                string           `yaml:"chain"`
	Chain                    *ChainConfig     `yaml:"-"`
	Address                  common.Address   `yaml:"address"`
	ValidatorContractAddress common.Address   `yaml:"validator_contract_address"`
	StartBlock               uint             `yaml:"start_block"`
	BlockConfirmations       uint             `yaml:"required_block_confirmations"`
	MaxBlockRangeSize        uint             `yaml:"max_block_range_size"`
	WhitelistedSenders       []common.Address `yaml:"whitelisted_senders"`
	ErcToNativeTokens        []TokenConfig    `yaml:"erc_to_native_tokens"`
}

type BridgeAlertConfig struct {
	HomeStartBlock    uint `yaml:"home_start_block"`
	ForeignStartBlock uint `yaml:"foreign_start_block"`
}

type BridgeMode string

const (
	BridgeModeArbitraryMessage BridgeMode = "AMB"
	BridgeModeErcToNative      BridgeMode = "ERC_TO_NATIVE"
)

type BridgeConfig struct {
	ID         string                        `yaml:"-"`
	BridgeMode BridgeMode                    `yaml:"bridge_mode"`
	Home       *BridgeSideConfig             `yaml:"home"`
	Foreign    *BridgeSideConfig             `yaml:"foreign"`
	Alerts     map[string]*BridgeAlertConfig `yaml:"alerts"`
}

type DBConfig struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	DB       string `yaml:"database"`
}

type PresenterConfig struct {
	Host string `yaml:"host"`
}

type Config struct {
	Chains          map[string]*ChainConfig  `yaml:"chains"`
	Bridges         map[string]*BridgeConfig `yaml:"bridges"`
	DBConfig        *DBConfig                `yaml:"postgres"`
	LogLevel        logrus.Level             `yaml:"log_level"`
	DisabledBridges []string                 `yaml:"disabled_bridges"`
	EnabledBridges  []string                 `yaml:"enabled_bridges"`
	Presenter       *PresenterConfig         `yaml:"presenter"`
}

func readYamlConfig(cfg *Config) error {
	blob, err := os.ReadFile("config.yml")
	if err != nil {
		return fmt.Errorf("can't access config file: %w", err)
	}
	blob = []byte(os.ExpandEnv(string(blob)))

	dec := yaml.NewDecoder(bytes.NewReader(blob))
	dec.KnownFields(true)
	err = dec.Decode(cfg)
	if err != nil {
		return fmt.Errorf("can't parse yaml config: %w", err)
	}
	return nil
}

func (cfg *Config) init() error {
	for bridgeID, bridge := range cfg.Bridges {
		bridge.ID = bridgeID
		err := bridge.Home.init(cfg)
		if err != nil {
			return fmt.Errorf("can't init home bridge config for %s: %w", bridgeID, err)
		}
		err = bridge.Foreign.init(cfg)
		if err != nil {
			return fmt.Errorf("can't init foreign bridge config for %s: %w", bridgeID, err)
		}
		if len(bridge.Home.ErcToNativeTokens) > 0 {
			return fmt.Errorf("%s home config error: %w", bridgeID, ErrNonEmptyTokenList)
		}
		if bridge.BridgeMode == BridgeModeErcToNative {
			if len(bridge.Foreign.ErcToNativeTokens) == 0 {
				return fmt.Errorf("%s foreign config error: %w", bridgeID, ErrEmptyTokenList)
			}
		} else {
			if len(bridge.Foreign.ErcToNativeTokens) > 0 {
				return fmt.Errorf("%s foreign config error: %w", bridgeID, ErrNonEmptyTokenList)
			}
			bridge.BridgeMode = BridgeModeArbitraryMessage
		}
	}
	return nil
}

func (cfg *BridgeSideConfig) init(parent *Config) error {
	if cfg.MaxBlockRangeSize <= 0 {
		cfg.MaxBlockRangeSize = 1000
	}
	chainName := cfg.ChainName
	var ok bool
	cfg.Chain, ok = parent.Chains[chainName]
	if !ok {
		return fmt.Errorf("unknown chain in config %q: %w", chainName, ErrInvalidConfig)
	}
	for i, tokenCfg := range cfg.ErcToNativeTokens {
		if tokenCfg.StartBlock == 0 {
			cfg.ErcToNativeTokens[i].StartBlock = cfg.StartBlock
		}
		if tokenCfg.EndBlock == 0 {
			cfg.ErcToNativeTokens[i].EndBlock = math.MaxUint32
		}
	}
	return nil
}

func (cfg *BridgeSideConfig) ContractAddresses(fromBlock, toBlock uint) []common.Address {
	return append(cfg.ErcToNativeTokenAddresses(fromBlock, toBlock), cfg.Address, cfg.ValidatorContractAddress)
}

func (cfg *BridgeSideConfig) ErcToNativeTokenAddresses(fromBlock, toBlock uint) []common.Address {
	addresses := make([]common.Address, 0, len(cfg.ErcToNativeTokens))
	for _, token := range cfg.ErcToNativeTokens {
		if toBlock < token.StartBlock || fromBlock > token.EndBlock {
			continue
		}
		addresses = append(addresses, token.Address)
	}
	return addresses
}

func ReadConfig() (*Config, error) {
	cfg := new(Config)
	err := readYamlConfig(cfg)
	if err != nil {
		return nil, err
	}
	err = cfg.init()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
