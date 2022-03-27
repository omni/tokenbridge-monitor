package config

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type RPCConfig struct {
	Host    string        `yaml:"host"`
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

type ReloadJobConfig struct {
	Event      string `yaml:"event"`
	StartBlock uint   `yaml:"start_block"`
	EndBlock   uint   `yaml:"end_block"`
}

type BridgeSideConfig struct {
	ChainName                string             `yaml:"chain"`
	Chain                    *ChainConfig       `yaml:"-"`
	Address                  common.Address     `yaml:"address"`
	ValidatorContractAddress common.Address     `yaml:"validator_contract_address"`
	StartBlock               uint               `yaml:"start_block"`
	BlockConfirmations       uint               `yaml:"required_block_confirmations"`
	MaxBlockRangeSize        uint               `yaml:"max_block_range_size"`
	RefetchEvents            []*ReloadJobConfig `yaml:"refetch_events"`
	WhitelistedSenders       []common.Address   `yaml:"whitelisted_senders"`
}

type BridgeAlertConfig struct {
	HomeStartBlock    uint `yaml:"home_start_block"`
	ForeignStartBlock uint `yaml:"foreign_start_block"`
}

type BridgeConfig struct {
	ID      string                        `yaml:"-"`
	Home    *BridgeSideConfig             `yaml:"home"`
	Foreign *BridgeSideConfig             `yaml:"foreign"`
	Alerts  map[string]*BridgeAlertConfig `yaml:"alerts"`
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
	f, err := os.ReadFile("config.yml")
	if err != nil {
		return fmt.Errorf("can't access config file: %w", err)
	}
	f = []byte(os.ExpandEnv(string(f)))

	dec := yaml.NewDecoder(bytes.NewReader(f))
	dec.KnownFields(true)
	err = dec.Decode(cfg)
	if err != nil {
		return fmt.Errorf("can't parse yaml config: %w", err)
	}
	return nil
}

func (cfg *Config) init() error {
	for id, bridge := range cfg.Bridges {
		bridge.ID = id
		if bridge.Home.MaxBlockRangeSize <= 0 {
			bridge.Home.MaxBlockRangeSize = 1000
		}
		if bridge.Foreign.MaxBlockRangeSize <= 0 {
			bridge.Foreign.MaxBlockRangeSize = 1000
		}
		for _, side := range [2]*BridgeSideConfig{bridge.Home, bridge.Foreign} {
			chainName := side.ChainName
			var ok bool
			side.Chain, ok = cfg.Chains[chainName]
			if !ok {
				return fmt.Errorf("unknown chain in config %q", chainName)
			}
		}
	}
	return nil
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
