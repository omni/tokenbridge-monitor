package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/ethclient"
	"github.com/poanetwork/tokenbridge-monitor/logging"
	"github.com/poanetwork/tokenbridge-monitor/monitor"
	"github.com/poanetwork/tokenbridge-monitor/presenter"
	"github.com/poanetwork/tokenbridge-monitor/repository"
)

func main() {
	logger := logging.New()

	cfg, err := config.ReadConfigFromFile("config.yml")
	if err != nil {
		logger.WithError(err).Fatal("can't read config")
	}
	logger.SetLevel(cfg.LogLevel)

	dbConn, err := db.ConnectToDBAndMigrate(cfg.DBConfig)
	if err != nil {
		logger.WithError(err).Fatal("can't connect to database and apply migrations")
	}
	defer dbConn.Close()

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(":2112", nil)
		if err != nil {
			logger.WithError(err).Fatal("can't start listener for prometheus metrics")
		}
	}()

	repo := repository.NewRepo(dbConn)
	if cfg.Presenter != nil {
		pr := presenter.NewPresenter(logger.WithField("service", "presenter"), repo, cfg)
		go func() {
			err := pr.Serve(cfg.Presenter.Host)
			if err != nil {
				logger.WithError(err).Fatal("can't serve presenter")
			}
		}()
	}

	monitors := make([]*monitor.Monitor, 0, len(cfg.Bridges))
	ctx, cancel := context.WithCancel(context.Background())
	for _, bridge := range cfg.DisabledBridges {
		delete(cfg.Bridges, bridge)
	}
	if cfg.EnabledBridges != nil {
		newBridgeCfg := make(map[string]*config.BridgeConfig, len(cfg.EnabledBridges))
		for _, bridge := range cfg.EnabledBridges {
			newBridgeCfg[bridge] = cfg.Bridges[bridge]
		}
		cfg.Bridges = newBridgeCfg
	}
	for _, bridgeCfg := range cfg.Bridges {
		bridgeLogger := logger.WithField("bridge_id", bridgeCfg.ID)
		homeClient, err2 := ethclient.NewClient(bridgeCfg.Home.Chain.RPC.Host, bridgeCfg.Home.Chain.RPC.Timeout, bridgeCfg.Home.Chain.ChainID)
		if err2 != nil {
			bridgeLogger.WithError(err2).Fatal("can't dial home rpc client")
		}
		foreignClient, err2 := ethclient.NewClient(bridgeCfg.Foreign.Chain.RPC.Host, bridgeCfg.Foreign.Chain.RPC.Timeout, bridgeCfg.Foreign.Chain.ChainID)
		if err2 != nil {
			bridgeLogger.WithError(err2).Fatal("can't dial foreign rpc client")
		}
		m, err2 := monitor.NewMonitor(ctx, bridgeLogger, dbConn, repo, bridgeCfg, homeClient, foreignClient)
		if err2 != nil {
			bridgeLogger.WithError(err2).Fatal("can't initialize bridge monitor")
		}

		monitors = append(monitors, m)
	}

	for _, m := range monitors {
		m.Start(ctx)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	for range c {
		cancel()
		logger.Warn("caught CTRL-C, gracefully terminating")
		return
	}
}
