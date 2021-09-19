package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
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
}

type BridgeSideConfig struct {
	ChainName          string         `yaml:"chain"`
	Chain              *ChainConfig   `yaml:"-"`
	Address            common.Address `yaml:"address"`
	StartBlock         uint           `yaml:"start_block"`
	BlockConfirmations uint           `yaml:"required_block_confirmations"`
	MaxBlockRangeSize  uint           `yaml:"max_block_range_size"`
}

type BridgeConfig struct {
	ID      string            `yaml:"-"`
	Home    *BridgeSideConfig `yaml:"home"`
	Foreign *BridgeSideConfig `yaml:"foreign"`
}

type DBConfig struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	DB       string `yaml:"database"`
}

type Config struct {
	Chains   map[string]*ChainConfig  `yaml:"chains"`
	Bridges  map[string]*BridgeConfig `yaml:"bridges"`
	DBConfig *DBConfig                `yaml:"postgres"`
}

func readYamlConfig(cfg *Config) error {
	f, err := ioutil.ReadFile("config.yml")
	if err != nil {
		return fmt.Errorf("can't access config file: %w", err)
	}
	f = []byte(os.ExpandEnv(string(f)))

	err = yaml.Unmarshal(f, cfg)
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
