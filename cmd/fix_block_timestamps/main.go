package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/entity"
	"github.com/poanetwork/tokenbridge-monitor/ethclient"
	"github.com/poanetwork/tokenbridge-monitor/logging"
	"github.com/poanetwork/tokenbridge-monitor/repository"
)

func main() {
	logger := logging.New()

	cfg, err := config.ReadConfigFromFile("config.yml")
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

	repo := repository.NewRepo(dbConn)

	query := `
DELETE
FROM block_timestamps bt
WHERE NOT exists(SELECT * FROM logs WHERE logs.chain_id = bt.chain_id AND logs.block_number = bt.block_number)`
	res, err := dbConn.ExecContext(context.Background(), query)
	if err != nil {
		logger.WithError(err).Fatal("can't delete unneeded data points")
	}
	n, _ := res.RowsAffected()
	logger.WithField("count", n).Infof("deleted unneeded block_timestamps records")

	query = `
SELECT *
FROM logs
WHERE not exists(SELECT *
                 FROM block_timestamps bt
                 WHERE logs.chain_id = bt.chain_id AND logs.block_number = bt.block_number)`
	logs := make([]*entity.Log, 0, 10)
	err = dbConn.SelectContext(context.Background(), &logs, query)
	if err != nil {
		logger.WithError(err).Fatal("can't select logs with missing block timestamps")
	}
	logger.WithField("count", len(logs)).Info("found logs records without associated block timestamp")

	bts := make(map[string]*entity.BlockTimestamp, len(logs))
	for _, log := range logs {
		bts[fmt.Sprintf("%s-%d", log.ChainID, log.BlockNumber)] = &entity.BlockTimestamp{
			ChainID:     log.ChainID,
			BlockNumber: log.BlockNumber,
		}
	}
	logger.WithField("count", len(bts)).Info("found block_timestamps to process")
	i := 0
	clients := make(map[string]ethclient.Client)
	for _, bt := range bts {
		fields := logrus.Fields{
			"chain_id": bt.ChainID,
		}
		if i%50 == 0 {
			logger.WithFields(logrus.Fields{
				"current": i,
				"total":   len(bts),
			}).Info("processing block_timestamp")
		}
		client, ok := clients[bt.ChainID]
		if !ok {
			chainCfg := cfg.GetChainConfig(bt.ChainID)
			if chainCfg == nil {
				logger.WithFields(fields).Fatal("can't find chain config")
			}
			client, err = ethclient.NewClient(chainCfg.RPC.Host, chainCfg.RPC.Timeout, chainCfg.ChainID)
			if err != nil {
				logger.WithFields(fields).WithError(err).Fatal("can't dial chain json rpc")
			}
			clients[bt.ChainID] = client
		}

		fields["block_number"] = bt.BlockNumber
		header, err2 := client.HeaderByNumber(context.Background(), bt.BlockNumber)
		if err2 != nil {
			logger.WithFields(fields).WithError(err2).Fatal("can't get block header")
		}
		bt.Timestamp = time.Unix(int64(header.Time), 0)
		err = repo.BlockTimestamps.Ensure(context.Background(), bt)
		if err != nil {
			logger.WithFields(fields).WithError(err).Fatal("can't insert block timestamp")
		}
		i++
	}
}
