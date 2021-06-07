package config

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

type RPCConfig struct {
	Host    string `yaml:"host"`
	Timeout int64  `yaml:"timeout"`
}

type ChainConfig struct {
	RPC       *RPCConfig `yaml:"rpc"`
	BlockTime uint64     `yaml:"block_time"`
}

type BridgeSideConfig struct {
	ChainName                  string       `yaml:"chain"`
	Chain                      *ChainConfig `yaml:"-"`
	Address                    string       `yaml:"address"`
	StartBlock                 uint64       `yaml:"start_block"`
	RequiredBlockConfirmations uint64       `yaml:"required_block_confirmations"`
	ManualBlockRanges          [][2]uint64  `yaml:"manual_block_ranges"`
}

type BridgeConfig struct {
	Id      string            `yaml:"-"`
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

func exit(err error, code int) {
	fmt.Println(err)
	os.Exit(code)
}

func exitOnError(err error, code int) {
	if err != nil {
		exit(err, code)
	}
}

func readYamlConfig(cfg *Config) {
	f, err := ioutil.ReadFile("config.yml")
	exitOnError(err, 1)
	f = []byte(os.ExpandEnv(string(f)))

	err = yaml.UnmarshalStrict(f, cfg)
	exitOnError(err, 2)
}

func processConfig(cfg *Config) {
	for id, bridge := range cfg.Bridges {
		bridge.Id = id
		for _, side := range [2]*BridgeSideConfig{bridge.Home, bridge.Foreign} {
			chainName := side.ChainName
			var ok bool
			side.Chain, ok = cfg.Chains[chainName]
			if !ok {
				exit(fmt.Errorf("unknown chain %s", chainName), 3)
			}
		}
	}
}

func ReadConfig() Config {
	var cfg Config
	readYamlConfig(&cfg)
	processConfig(&cfg)
	return cfg
}
