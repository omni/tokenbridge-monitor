package monitor

import (
	"amb-monitor/config"
	"amb-monitor/contract"
	"amb-monitor/contract/constants"
	"amb-monitor/db"
	"amb-monitor/entity"
	"amb-monitor/ethclient"
	"amb-monitor/logging"
	"amb-monitor/monitor/alerts"
	"amb-monitor/repository"
	"amb-monitor/utils"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type ContractMonitor struct {
	cfg                  *config.BridgeSideConfig
	logger               logging.Logger
	repo                 *repository.Repo
	client               *ethclient.Client
	logsCursor           *entity.LogsCursor
	blocksRangeChan      chan *BlocksRange
	logsChan             chan *LogsBatch
	contract             *contract.Contract
	eventHandlers        map[string]EventHandler
	headBlock            uint
	syncedMetric         prometheus.Gauge
	headBlockMetric      prometheus.Gauge
	fetchedBlockMetric   prometheus.Gauge
	processedBlockMetric prometheus.Gauge
}

type Monitor struct {
	cfg            *config.BridgeConfig
	logger         logging.Logger
	repo           *repository.Repo
	homeMonitor    *ContractMonitor
	foreignMonitor *ContractMonitor
	alertManager   *alerts.AlertManager
}

const defaultSyncedThreshold = 10

func newContractMonitor(ctx context.Context, logger logging.Logger, repo *repository.Repo, bridge string, cfg *config.BridgeSideConfig) (*ContractMonitor, error) {
	client, err := ethclient.NewClient(cfg.Chain.RPC.Host, cfg.Chain.RPC.Timeout, cfg.Chain.ChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to start eth client: %w", err)
	}
	logsCursor, err := repo.LogsCursors.GetByChainIDAndAddress(ctx, client.ChainID, cfg.Address)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.WithFields(logrus.Fields{
				"chain_id":    client.ChainID,
				"address":     cfg.Address,
				"start_block": cfg.StartBlock,
			}).Warn("contract cursor is not present, staring indexing from scratch")
			logsCursor = &entity.LogsCursor{
				ChainID:            client.ChainID,
				Address:            cfg.Address,
				LastFetchedBlock:   cfg.StartBlock - 1,
				LastProcessedBlock: cfg.StartBlock - 1,
			}
		} else {
			return nil, fmt.Errorf("failed to read home logs cursor: %w", err)
		}
	}
	commonLabels := prometheus.Labels{
		"bridge_id": bridge,
		"chain_id":  client.ChainID,
		"address":   cfg.Address.String(),
	}
	return &ContractMonitor{
		logger:               logger,
		cfg:                  cfg,
		repo:                 repo,
		client:               client,
		logsCursor:           logsCursor,
		blocksRangeChan:      make(chan *BlocksRange, 10),
		logsChan:             make(chan *LogsBatch, 200),
		contract:             contract.NewContract(client, cfg.Address, constants.AMB),
		eventHandlers:        make(map[string]EventHandler, 10),
		syncedMetric:         SyncedContract.With(commonLabels),
		headBlockMetric:      LatestHeadBlock.With(commonLabels),
		fetchedBlockMetric:   LatestFetchedBlock.With(commonLabels),
		processedBlockMetric: LatestProcessedBlock.With(commonLabels),
	}, nil
}

func NewMonitor(ctx context.Context, logger logging.Logger, dbConn *db.DB, repo *repository.Repo, cfg *config.BridgeConfig) (*Monitor, error) {
	homeMonitor, err := newContractMonitor(ctx, logger.WithField("contract", "home"), repo, cfg.ID, cfg.Home)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize home side monitor: %w", err)
	}
	foreignMonitor, err := newContractMonitor(ctx, logger.WithField("contract", "foreign"), repo, cfg.ID, cfg.Foreign)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize foreign side monitor: %w", err)
	}
	alertManager, err := alerts.NewAlertManager(logger, dbConn, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize alert manager: %w", err)
	}
	handlers := NewBridgeEventHandler(repo, cfg.ID, homeMonitor.client)
	homeMonitor.eventHandlers["UserRequestForSignature"] = handlers.HandleUserRequestForSignature
	homeMonitor.eventHandlers["UserRequestForSignature0"] = handlers.HandleLegacyUserRequestForSignature
	homeMonitor.eventHandlers["SignedForUserRequest"] = handlers.HandleSignedForUserRequest
	homeMonitor.eventHandlers["SignedForAffirmation"] = handlers.HandleSignedForAffirmation
	homeMonitor.eventHandlers["AffirmationCompleted"] = handlers.HandleAffirmationCompleted
	homeMonitor.eventHandlers["AffirmationCompleted0"] = handlers.HandleAffirmationCompleted
	homeMonitor.eventHandlers["CollectedSignatures"] = handlers.HandleCollectedSignatures
	homeMonitor.eventHandlers["UserRequestForInformation"] = handlers.HandleUserRequestForInformation
	homeMonitor.eventHandlers["SignedForInformation"] = handlers.HandleSignedForInformation
	homeMonitor.eventHandlers["InformationRetrieved"] = handlers.HandleInformationRetrieved
	foreignMonitor.eventHandlers["UserRequestForAffirmation"] = handlers.HandleUserRequestForAffirmation
	foreignMonitor.eventHandlers["UserRequestForAffirmation0"] = handlers.HandleLegacyUserRequestForAffirmation
	foreignMonitor.eventHandlers["RelayedMessage"] = handlers.HandleRelayedMessage
	foreignMonitor.eventHandlers["RelayedMessage0"] = handlers.HandleRelayedMessage
	return &Monitor{
		cfg:            cfg,
		logger:         logger,
		repo:           repo,
		homeMonitor:    homeMonitor,
		foreignMonitor: foreignMonitor,
		alertManager:   alertManager,
	}, nil
}

func (m *Monitor) Start(ctx context.Context) {
	m.logger.Info("starting bridge monitor")
	go m.homeMonitor.Start(ctx)
	go m.foreignMonitor.Start(ctx)
	go m.alertManager.Start(ctx, m.IsSynced)
}

func (m *Monitor) IsSynced() bool {
	return m.homeMonitor.IsSynced() && m.foreignMonitor.IsSynced()
}

func (m *ContractMonitor) IsSynced() bool {
	if m.headBlock > 0 && m.logsCursor.LastProcessedBlock+defaultSyncedThreshold > m.headBlock {
		m.syncedMetric.Set(1)
		return true
	}
	m.syncedMetric.Set(0)
	return false
}

func (m *ContractMonitor) Start(ctx context.Context) {
	lastProcessedBlock := m.logsCursor.LastProcessedBlock
	lastFetchedBlock := m.logsCursor.LastFetchedBlock
	go m.StartBlockFetcher(ctx, lastFetchedBlock+1)
	go m.StartLogsProcessor(ctx)
	m.LoadUnprocessedLogs(ctx, m.cfg.ReloadEvents, lastProcessedBlock+1, lastFetchedBlock)
	go m.StartLogsFetcher(ctx)
}

func (m *ContractMonitor) LoadUnprocessedLogs(ctx context.Context, reloadEvents []string, fromBlock, toBlock uint) {
	m.logger.WithFields(logrus.Fields{
		"from_block":          fromBlock,
		"to_block":            toBlock,
		"manual_reload_count": len(reloadEvents),
	}).Info("loading fetched but not yet processed blocks")

	for _, event := range reloadEvents {
		topic := crypto.Keccak256Hash([]byte(event))

		m.logger.WithFields(logrus.Fields{
			"from_block": m.cfg.StartBlock,
			"to_block":   fromBlock,
			"event":      event,
			"topic":      topic,
		}).Info("manually reloading events")
		for {
			logs, err := m.repo.Logs.FindByTopicAndBlockRange(ctx, m.client.ChainID, m.cfg.Address, m.cfg.StartBlock, fromBlock, topic)
			if err != nil {
				m.logger.WithError(err).Error("can't find manually reloaded logs in block range")
				if utils.ContextSleep(ctx, 10*time.Second) == nil {
					return
				}
				continue
			}

			m.submitLogs(logs, toBlock)
			break
		}
	}

	var logs []*entity.Log
	for {
		var err error
		logs, err = m.repo.Logs.FindByBlockRange(ctx, m.client.ChainID, m.cfg.Address, fromBlock, toBlock)
		if err != nil {
			m.logger.WithError(err).Error("can't find unprocessed logs in block range")
			if utils.ContextSleep(ctx, 10*time.Second) == nil {
				return
			}
			continue
		}
		break
	}

	m.submitLogs(logs, toBlock)
}

func (m *ContractMonitor) StartBlockFetcher(ctx context.Context, start uint) {
	m.logger.Info("starting new blocks tracker")

	for {
		head, err := m.client.BlockNumber(ctx)
		if err != nil {
			m.logger.WithError(err).Error("can't fetch latest block number")
		} else {
			m.headBlock = uint(head) - m.cfg.BlockConfirmations

			if start > m.headBlock {
				m.logger.WithFields(logrus.Fields{
					"head_block":                   m.headBlock,
					"required_block_confirmations": m.cfg.BlockConfirmations,
					"current_block":                start,
				}).Warn("latest block is behind processed block in the database, skipping")
			}
			m.headBlockMetric.Set(float64(m.headBlock))

			for start <= m.headBlock {
				end := start + m.cfg.MaxBlockRangeSize - 1
				if end > m.headBlock {
					end = m.headBlock
				}
				m.logger.WithFields(logrus.Fields{
					"from_block": start,
					"to_block":   end,
				}).Info("scheduling new block range logs search")
				m.blocksRangeChan <- &BlocksRange{
					From: start,
					To:   end,
				}
				start = end + 1
			}
		}

		if utils.ContextSleep(ctx, m.cfg.Chain.BlockIndexInterval) == nil {
			return
		}
	}
}

func (m *ContractMonitor) StartLogsFetcher(ctx context.Context) {
	m.logger.Info("starting logs fetcher")
	for {
		select {
		case <-ctx.Done():
			return
		case blocksRange := <-m.blocksRangeChan:
			for {
				err := m.tryToFetchLogs(ctx, blocksRange)
				if err != nil {
					m.logger.WithError(err).WithFields(logrus.Fields{
						"from_block": blocksRange.From,
						"to_block":   blocksRange.To,
					}).Error("failed logs fetching, retrying")
					continue
				}
				break
			}
		}
	}
}

func (m *ContractMonitor) tryToFetchLogs(ctx context.Context, blocksRange *BlocksRange) error {
	logs, err := m.client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(blocksRange.From)),
		ToBlock:   big.NewInt(int64(blocksRange.To)),
		Addresses: []common.Address{m.cfg.Address},
	})
	if err != nil {
		return err
	}
	m.logger.WithFields(logrus.Fields{
		"count":      len(logs),
		"from_block": blocksRange.From,
		"to_block":   blocksRange.To,
	}).Info("fetched logs in range")
	entities := make([]*entity.Log, len(logs))
	if len(logs) > 0 {
		sort.Slice(logs, func(i, j int) bool {
			a, b := &logs[i], &logs[j]
			return a.BlockNumber < b.BlockNumber || (a.BlockNumber == b.BlockNumber && a.Index < b.Index)
		})
		for i, log := range logs {
			entities[i] = m.logToEntity(log)
		}
		err = m.repo.Logs.Ensure(ctx, entities...)
		if err != nil {
			return err
		}

		indexes := make([]uint, len(entities))
		for i, x := range entities {
			indexes[i] = x.ID
		}
		m.logger.WithFields(logrus.Fields{
			"count":      len(logs),
			"from_block": blocksRange.From,
			"to_block":   blocksRange.To,
		}).Info("saved logs")
	}
	if err = m.recordFetchedBlockNumber(ctx, blocksRange.To); err != nil {
		return err
	}

	m.submitLogs(entities, blocksRange.To)
	return nil
}

func (m *ContractMonitor) submitLogs(logs []*entity.Log, endBlock uint) {
	jobs, lastBlock := 0, uint(0)
	for _, log := range logs {
		if log.BlockNumber > lastBlock {
			lastBlock = log.BlockNumber
			jobs++
		}
	}
	m.logger.WithFields(logrus.Fields{
		"count": len(logs),
		"jobs":  jobs,
	}).Info("create jobs for logs processor")
	// fake log to simplify loop, it will be skipped
	logs = append(logs, &entity.Log{BlockNumber: math.MaxUint32})
	batchStartIndex := 0
	for i, log := range logs {
		if log.BlockNumber > logs[batchStartIndex].BlockNumber {
			m.logger.WithFields(logrus.Fields{
				"count":        i - batchStartIndex,
				"block_number": logs[batchStartIndex].BlockNumber,
			}).Debug("submitting logs batch to logs processor")
			m.logsChan <- &LogsBatch{
				BlockNumber: logs[batchStartIndex].BlockNumber,
				Logs:        logs[batchStartIndex:i],
			}
			batchStartIndex = i
		}
	}
	if lastBlock < endBlock {
		m.logsChan <- &LogsBatch{
			BlockNumber: endBlock,
			Logs:        nil,
		}
	}
}

func (m *ContractMonitor) logToEntity(log types.Log) *entity.Log {
	e := &entity.Log{
		ChainID:         m.cfg.Chain.ChainID,
		Address:         log.Address,
		Data:            log.Data,
		BlockNumber:     uint(log.BlockNumber),
		LogIndex:        log.Index,
		TransactionHash: log.TxHash,
	}
	if len(log.Topics) > 0 {
		e.Topic0 = &log.Topics[0]
		if len(log.Topics) > 1 {
			e.Topic1 = &log.Topics[1]
			if len(log.Topics) > 2 {
				e.Topic2 = &log.Topics[2]
				if len(log.Topics) > 3 {
					e.Topic3 = &log.Topics[3]
				}
			}
		}
	}
	return e
}

func (m *ContractMonitor) StartLogsProcessor(ctx context.Context) {
	m.logger.Info("starting logs processor")
	for {
		select {
		case <-ctx.Done():
			return
		case logs := <-m.logsChan:
			wg := new(sync.WaitGroup)
			wg.Add(2)
			go func() {
				defer wg.Done()
				for {
					err := m.tryToGetBlockTimestamp(ctx, logs.BlockNumber)
					if err != nil {
						m.logger.WithError(err).WithFields(logrus.Fields{
							"block_number": logs.BlockNumber,
						}).Error("failed to get block timestamp, retrying")
						continue
					}
					return
				}
			}()

			go func() {
				defer wg.Done()
				for {
					err := m.tryToProcessLogsBatch(ctx, logs)
					if err != nil {
						m.logger.WithError(err).WithFields(logrus.Fields{
							"block_number": logs.BlockNumber,
							"count":        len(logs.Logs),
						}).Error("failed to process logs batch, retrying")
						continue
					}
					return
				}
			}()
			wg.Wait()

			for {
				err := m.recordProcessedBlockNumber(ctx, logs.BlockNumber)
				if err != nil {
					m.logger.WithError(err).WithField("block_number", logs.BlockNumber).
						Error("failed to update latest processed block number, retrying")
					if utils.ContextSleep(ctx, 10*time.Second) == nil {
						return
					}
					continue
				}
				break
			}
		}
	}
}

func (m *ContractMonitor) tryToGetBlockTimestamp(ctx context.Context, blockNumber uint) error {
	ts, err := m.repo.BlockTimestamps.GetByBlockNumber(ctx, m.client.ChainID, blockNumber)
	if err != nil {
		return err
	}
	if ts != nil {
		m.logger.WithField("block_number", blockNumber).Debug("timestamp already exists, skipping")
		return nil
	}
	m.logger.WithField("block_number", blockNumber).Debug("fetching block timestamp")
	header, err := m.client.HeaderByNumber(ctx, blockNumber)
	if err != nil {
		return err
	}
	return m.repo.BlockTimestamps.Ensure(ctx, &entity.BlockTimestamp{
		ChainID:     m.client.ChainID,
		BlockNumber: blockNumber,
		Timestamp:   time.Unix(int64(header.Time), 0),
	})
}

func (m *ContractMonitor) tryToProcessLogsBatch(ctx context.Context, logs *LogsBatch) error {
	m.logger.WithFields(logrus.Fields{
		"count":        len(logs.Logs),
		"block_number": logs.BlockNumber,
	}).Debug("processing logs batch")
	for _, log := range logs.Logs {
		event, data, err := m.contract.ParseLog(log)
		if err != nil {
			return fmt.Errorf("can't parse log: %w", err)
		}
		handle, ok := m.eventHandlers[event]
		if !ok {
			if event == "" {
				event = log.Topic0.String()
			}
			m.logger.WithFields(logrus.Fields{
				"event":        event,
				"log_id":       log.ID,
				"block_number": log.BlockNumber,
				"tx_hash":      log.TransactionHash,
				"log_index":    log.LogIndex,
			}).Warn("received unknown event")
			continue
		}
		m.logger.WithFields(logrus.Fields{
			"event":  event,
			"log_id": log.ID,
		}).Trace("handling event")
		if err = handle(ctx, log, data); err != nil {
			return err
		}
	}
	return nil
}

func (m *ContractMonitor) recordFetchedBlockNumber(ctx context.Context, blockNumber uint) error {
	if blockNumber < m.logsCursor.LastFetchedBlock {
		return nil
	}

	m.logsCursor.LastFetchedBlock = blockNumber
	err := m.repo.LogsCursors.Ensure(ctx, m.logsCursor)
	if err != nil {
		return err
	}
	m.fetchedBlockMetric.Set(float64(blockNumber))
	return nil
}

func (m *ContractMonitor) recordProcessedBlockNumber(ctx context.Context, blockNumber uint) error {
	if blockNumber < m.logsCursor.LastProcessedBlock {
		return nil
	}

	m.logsCursor.LastProcessedBlock = blockNumber
	err := m.repo.LogsCursors.Ensure(ctx, m.logsCursor)
	if err != nil {
		return err
	}
	m.processedBlockMetric.Set(float64(blockNumber))
	return nil
}
