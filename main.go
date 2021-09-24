package main

import (
	"amb-monitor/config"
	"amb-monitor/db"
	"amb-monitor/logging"
	"amb-monitor/monitor"
	"amb-monitor/repository"
	"context"
	"net/http"
	"os"
	"os/signal"

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
		m, err2 := monitor.NewMonitor(ctx, logger.WithField("bridge_id", bridgeCfg.ID), dbConn, repo, bridgeCfg)
		if err2 != nil {
			logger.WithError(err2).Fatal("can't initialize bridge monitor")
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
