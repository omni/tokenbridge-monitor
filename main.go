package main

import (
	"amb-monitor/config"
	"amb-monitor/db"
	"amb-monitor/ethclient"
	"amb-monitor/logging"
	"amb-monitor/monitor"
	"context"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	var logger = logging.GetLogger("main")

	cfg := config.ReadConfig()

	err := db.SanityCheck(cfg.DBConfig)
	if err != nil {
		logger.Fatal(err)
	}

	conn, err := db.ConnectDB(cfg.DBConfig)
	if err != nil {
		logger.Fatal(err)
	}

	clients := make(map[string]*ethclient.Client, len(cfg.Bridges)*2)

	for bridgeName, bridge := range cfg.Bridges {
		logger.Printf("initializing monitor state for bridge %s\n", bridgeName)
		state, err := monitor.NewState(bridge, conn)
		if err != nil {
			logger.Fatal(err)
		}
		clients[bridge.Home.ChainName] = state.Home.Client
		clients[bridge.Foreign.ChainName] = state.Foreign.Client

		logger.Printf("starting monitor for bridge %s\n", bridgeName)
		go state.Home.StartBlockWatcher(context.Background())
		go state.Home.StartEventFetcher(context.Background())
		go state.Home.StartLogDispatcher(context.Background(), conn)
		go state.Home.StartDbWriter(context.Background(), conn)

		go state.Foreign.StartBlockWatcher(context.Background())
		go state.Foreign.StartEventFetcher(context.Background())
		go state.Foreign.StartLogDispatcher(context.Background(), conn)
		go state.Foreign.StartDbWriter(context.Background(), conn)

		go state.StartReindexer(context.Background(), conn)
	}

	for chainId, client := range clients {
		fmt.Println(chainId, cfg.Chains)
		go monitor.StartBlockIndexer(context.Background(), conn, chainId, client, cfg.Chains[chainId])
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	for _ = range c {
		logger.Printf("caught CTRL-C, gracefully terminating")
		conn.Close()
		os.Exit(0)
	}
}
