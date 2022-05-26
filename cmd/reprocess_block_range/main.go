package main

import (
	"context"
	"flag"
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/ethclient"
	"github.com/poanetwork/tokenbridge-monitor/logging"
	"github.com/poanetwork/tokenbridge-monitor/monitor"
	"github.com/poanetwork/tokenbridge-monitor/repository"
)

var (
	bridgeID  = flag.String("bridgeId", "", "bridgeId to reprocess message in")
	home      = flag.Bool("home", false, "reprocess home messages")
	foreign   = flag.Bool("foreign", false, "reprocess foreign messages")
	fromBlock = flag.Uint("fromBlock", 0, "starting block")
	toBlock   = flag.Uint("toBlock", 0, "ending block")
)

func main() {
	flag.Parse()

	logger := logging.New()

	cfg, err := config.ReadConfigFromFile("config.yml")
	if err != nil {
		logger.WithError(err).Fatal("can't read config")
	}
	logger.SetLevel(cfg.LogLevel)

	if *bridgeID == "" {
		logger.Fatal("bridgeId is not specified")
	}
	if *home == *foreign {
		logger.Fatal("exactly one of --home or --foreign should be specified")
	}
	bridgeCfg, ok := cfg.Bridges[*bridgeID]
	if !ok || bridgeCfg == nil {
		logger.WithField("bridge_id", *bridgeID).Fatal("bridge config for given bridgeId is not found")
	}
	sideCfg := bridgeCfg.Foreign
	if *home {
		sideCfg = bridgeCfg.Home
	}
	if *fromBlock < sideCfg.StartBlock {
		fromBlock = &sideCfg.StartBlock
	}
	if *toBlock == 0 {
		logger.Fatal("toBlock is not specified")
	}
	if *toBlock < *fromBlock {
		logger.WithFields(logrus.Fields{
			"from_block": *fromBlock,
			"to_block":   *toBlock,
		}).Fatal("toBlock < fromBlock is not specified")
	}

	dbConn, err := db.NewDB(cfg.DBConfig)
	if err != nil {
		logger.WithError(err).Fatal("can't connect to database")
	}
	defer dbConn.Close()

	if err = dbConn.Migrate(); err != nil {
		logger.WithError(err).Fatal("can't run database migrations")
	}

	ctx, cancel := context.WithCancel(context.Background())
	repo := repository.NewRepo(dbConn)
	bridgeLogger := logger.WithField("bridge_id", bridgeCfg.ID)
	homeClient, err2 := ethclient.NewClient(bridgeCfg.Home.Chain.RPC.Host, bridgeCfg.Home.Chain.RPC.Timeout, bridgeCfg.Home.Chain.ChainID)
	if err2 != nil {
		bridgeLogger.WithError(err2).Fatal("can't dial home rpc client")
	}
	foreignClient, err2 := ethclient.NewClient(bridgeCfg.Foreign.Chain.RPC.Host, bridgeCfg.Foreign.Chain.RPC.Timeout, bridgeCfg.Foreign.Chain.ChainID)
	if err2 != nil {
		bridgeLogger.WithError(err2).Fatal("can't dial foreign rpc client")
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		for range c {
			cancel()
			logger.Warn("caught CTRL-C, gracefully terminating")
			return
		}
	}()

	m, err2 := monitor.NewMonitor(ctx, bridgeLogger, dbConn, repo, bridgeCfg, homeClient, foreignClient)
	if err2 != nil {
		bridgeLogger.WithError(err2).Fatal("can't initialize bridge monitor")
	}

	err = m.ProcessBlockRange(ctx, *home, *fromBlock, *toBlock)
	if err != nil {
		logger.WithError(err).Fatal("can't manually process block range")
	}
}
