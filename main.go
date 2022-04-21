package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"tokenbridge-monitor/config"
	"tokenbridge-monitor/db"
	"tokenbridge-monitor/logging"
	"tokenbridge-monitor/monitor"
	"tokenbridge-monitor/presenter"
	"tokenbridge-monitor/repository"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var logger = logging.New()

	cfg, err := config.ReadConfig()
	if err != nil {
		logger.WithError(err).Fatal("can't read config")
	}
	logger.SetLevel(cfg.LogLevel)

	dbConn, err := db.NewDB(cfg.DBConfig)
	if err != nil {
		logger.WithError(err).Fatal("can't connect to database")
	}
	defer dbConn.Close()

	if err = dbConn.Migrate(); err != nil {
		logger.WithError(err).Fatal("can't run database migrations")
	}

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(":2112", nil)
		if err != nil {
			logger.WithError(err).Fatal("can't start listener for prometheus metrics")
		}
	}()

	repo := repository.NewRepo(dbConn)
	if cfg.Presenter != nil {
		pr := presenter.NewPresenter(logger.WithField("service", "presenter"), repo, cfg.Bridges)
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
		m, err2 := monitor.NewMonitor(ctx, bridgeLogger, dbConn, repo, bridgeCfg)
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
		os.Exit(0)
	}
}
