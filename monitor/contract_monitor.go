package monitor

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/contract"
	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/entity"
	"github.com/poanetwork/tokenbridge-monitor/ethclient"
	"github.com/poanetwork/tokenbridge-monitor/logging"
	"github.com/poanetwork/tokenbridge-monitor/repository"
	"github.com/poanetwork/tokenbridge-monitor/utils"
)

const (
	defaultSyncedThreshold     = 10
	defaultBlockRangesChanCap  = 10
	defaultLogsChanCap         = 200
	defaultEventHandlersMapCap = 20
)

var ErrIncompatibleABI = errors.New("incompatible ABI")

type ContractMonitor struct {
	bridgeCfg            *config.BridgeConfig
	cfg                  *config.BridgeSideConfig
	logger               logging.Logger
	repo                 *repository.Repo
	client               ethclient.Client
	logsCursor           *entity.LogsCursor
	blocksRangeChan      chan *BlocksRange
	logsChan             chan *LogsBatch
	contract             *contract.BridgeContract
	eventHandlers        map[string]EventHandler
	headBlock            uint
	isSynced             bool
	syncedMetric         prometheus.Gauge
	headBlockMetric      prometheus.Gauge
	fetchedBlockMetric   prometheus.Gauge
	processedBlockMetric prometheus.Gauge
}

func NewContractMonitor(ctx context.Context, logger logging.Logger, repo *repository.Repo, bridgeCfg *config.BridgeConfig, cfg *config.BridgeSideConfig, client ethclient.Client) (*ContractMonitor, error) {
	bridgeContract := contract.NewBridgeContract(client, cfg.Address, bridgeCfg.BridgeMode)
	if cfg.ValidatorContractAddress == (common.Address{}) {
		addr, err := bridgeContract.ValidatorContractAddress(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot get validator contract address: %w", err)
		}
		logger.WithFields(logrus.Fields{
			"chain_id":                   cfg.Chain.ChainID,
			"bridge_address":             cfg.Address,
			"validator_contract_address": cfg.ValidatorContractAddress,
			"start_block":                cfg.StartBlock,
		}).Info("obtained validator contract address")
		cfg.ValidatorContractAddress = addr
	}
	logsCursor, err := repo.LogsCursors.GetByChainIDAndAddress(ctx, cfg.Chain.ChainID, cfg.Address)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			logger.WithFields(logrus.Fields{
				"chain_id":    cfg.Chain.ChainID,
				"address":     cfg.Address,
				"start_block": cfg.StartBlock,
			}).Warn("contract cursor is not present, staring indexing from scratch")
			logsCursor = &entity.LogsCursor{
				ChainID:            cfg.Chain.ChainID,
				Address:            cfg.Address,
				LastFetchedBlock:   cfg.StartBlock - 1,
				LastProcessedBlock: cfg.StartBlock - 1,
			}
		} else {
			return nil, fmt.Errorf("failed to read logs cursor: %w", err)
		}
	}
	commonLabels := prometheus.Labels{
		"bridge_id": bridgeCfg.ID,
		"chain_id":  cfg.Chain.ChainID,
		"address":   cfg.Address.String(),
	}
	return &ContractMonitor{
		logger:               logger,
		bridgeCfg:            bridgeCfg,
		cfg:                  cfg,
		repo:                 repo,
		client:               client,
		logsCursor:           logsCursor,
		blocksRangeChan:      make(chan *BlocksRange, defaultBlockRangesChanCap),
		logsChan:             make(chan *LogsBatch, defaultLogsChanCap),
		contract:             bridgeContract,
		eventHandlers:        make(map[string]EventHandler, defaultEventHandlersMapCap),
		syncedMetric:         SyncedContract.With(commonLabels),
		headBlockMetric:      LatestHeadBlock.With(commonLabels),
		fetchedBlockMetric:   LatestFetchedBlock.With(commonLabels),
		processedBlockMetric: LatestProcessedBlock.With(commonLabels),
	}, nil
}

func (m *ContractMonitor) IsSynced() bool {
	return m.isSynced
}

func (m *ContractMonitor) RegisterEventHandler(event string, handler EventHandler) {
	m.eventHandlers[event] = handler
}

func (m *ContractMonitor) VerifyEventHandlersABI() error {
	events := m.contract.ABI.AllEvents()
	for e := range m.eventHandlers {
		if !events[e] {
			return fmt.Errorf("contract does not have %s event in its ABI: %w", e, ErrIncompatibleABI)
		}
	}
	return nil
}

func (m *ContractMonitor) Start(ctx context.Context) {
	lastProcessedBlock := m.logsCursor.LastProcessedBlock
	lastFetchedBlock := m.logsCursor.LastFetchedBlock
	m.processedBlockMetric.Set(float64(lastProcessedBlock))
	m.fetchedBlockMetric.Set(float64(lastFetchedBlock))
	go m.StartBlockFetcher(ctx, lastFetchedBlock+1)
	go m.StartLogsProcessor(ctx)
	m.LoadUnprocessedLogs(ctx, lastProcessedBlock+1, lastFetchedBlock)
	go m.StartLogsFetcher(ctx)
}

//nolint:cyclop
func (m *ContractMonitor) ProcessBlockRange(ctx context.Context, fromBlock, toBlock uint) error {
	if toBlock > m.logsCursor.LastProcessedBlock {
		return fmt.Errorf("can't manually process logs further then current lastProcessedBlock: %w", config.ErrInvalidConfig)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	m.blocksRangeChan <- nil
	go func() {
		defer wg.Done()

		batches := SplitBlockRange(fromBlock, toBlock, m.cfg.MaxBlockRangeSize)
		for _, batch := range batches {
			m.logger.WithFields(logrus.Fields{
				"from_block": batch.From,
				"to_block":   batch.To,
			}).Info("scheduling new block range logs search")
			select {
			case m.blocksRangeChan <- batch:
			case <-ctx.Done():
				return
			}
		}
		m.blocksRangeChan <- nil
	}()

	go m.StartLogsFetcher(ctx)
	go m.StartLogsProcessor(ctx)

	finishedFetching := false
	for {
		if !finishedFetching && len(m.blocksRangeChan) == 0 {
			// last nil from m.blocksRangeChan was consumed, meaning that all previous values were already handled
			// and log batches were sent to processing queue
			m.logger.Info("all block ranges were processed, submitting stub logs batch")
			finishedFetching = true
			m.logsChan <- nil
		}
		if finishedFetching && len(m.logsChan) == 0 {
			// last nil from m.logsChan was consumed, meaning that all previous values were already processed
			// there is nothing to process, so exit
			m.logger.Info("all logs batches were processed, exiting")
			return nil
		}

		if utils.ContextSleep(ctx, time.Second) == nil {
			return ctx.Err()
		}
	}
}

func (m *ContractMonitor) LoadUnprocessedLogs(ctx context.Context, fromBlock, toBlock uint) {
	m.logger.WithFields(logrus.Fields{
		"from_block": fromBlock,
		"to_block":   toBlock,
	}).Info("loading fetched but not yet processed blocks")

	filter := entity.LogsFilter{
		ChainID:   &m.cfg.Chain.ChainID,
		Addresses: m.cfg.ContractAddresses(fromBlock, toBlock),
		FromBlock: &fromBlock,
		ToBlock:   &toBlock,
	}
	for {
		logs, err := m.repo.Logs.Find(ctx, filter)
		if err != nil {
			m.logger.WithError(err).Error("can't find unprocessed logs in block range")
		} else {
			m.submitLogs(logs, toBlock)
			break
		}

		if utils.ContextSleep(ctx, 10*time.Second) == nil {
			return
		}
	}
}

func (m *ContractMonitor) StartBlockFetcher(ctx context.Context, start uint) {
	m.logger.Info("starting new blocks tracker")

	for {
		head, err := m.client.BlockNumber(ctx)
		if err != nil {
			m.logger.WithError(err).Error("can't fetch latest block number")
		} else {
			head -= m.cfg.BlockConfirmations
			m.recordHeadBlockNumber(head)

			batches := SplitBlockRange(start, head, m.cfg.MaxBlockRangeSize)
			for _, batch := range batches {
				m.logger.WithFields(logrus.Fields{
					"from_block": batch.From,
					"to_block":   batch.To,
				}).Info("scheduling new block range logs search")
				m.blocksRangeChan <- batch
			}
			start = head + 1
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
			if blocksRange == nil {
				continue
			}
			for {
				err := m.tryToFetchLogs(ctx, blocksRange)
				if err != nil {
					m.logger.WithError(err).WithFields(logrus.Fields{
						"from_block": blocksRange.From,
						"to_block":   blocksRange.To,
					}).Error("failed logs fetching, retrying")
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

func (m *ContractMonitor) buildFilterQueries(blocksRange *BlocksRange) []ethereum.FilterQuery {
	var queries []ethereum.FilterQuery
	q := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(blocksRange.From)),
		ToBlock:   big.NewInt(int64(blocksRange.To)),
		Addresses: []common.Address{m.cfg.Address, m.cfg.ValidatorContractAddress},
	}
	queries = append(queries, q)
	if m.bridgeCfg.BridgeMode == config.BridgeModeErcToNative {
		for _, token := range m.cfg.ErcToNativeTokens {
			if blocksRange.To < token.StartBlock || blocksRange.From > token.EndBlock {
				continue
			}
			q = ethereum.FilterQuery{
				FromBlock: big.NewInt(int64(blocksRange.From)),
				ToBlock:   big.NewInt(int64(blocksRange.To)),
				Addresses: []common.Address{token.Address},
				Topics:    [][]common.Hash{{}, {}, {m.cfg.Address.Hash()}},
			}
			if token.StartBlock > blocksRange.From {
				q.FromBlock = big.NewInt(int64(token.StartBlock))
			}
			if token.EndBlock < blocksRange.To {
				q.ToBlock = big.NewInt(int64(token.EndBlock))
			}
			queries = append(queries, q)
		}
	}
	return queries
}

func (m *ContractMonitor) tryToFetchLogs(ctx context.Context, blocksRange *BlocksRange) error {
	qs := m.buildFilterQueries(blocksRange)
	var logs []*entity.Log
	var logsBatch []types.Log
	var err error
	for _, q := range qs {
		if m.cfg.Chain.SafeLogsRequest {
			logsBatch, err = m.client.FilterLogsSafe(ctx, q)
		} else {
			logsBatch, err = m.client.FilterLogs(ctx, q)
		}
		if err != nil {
			return err
		}
		for _, log := range logsBatch {
			logs = append(logs, entity.NewLog(m.cfg.Chain.ChainID, log))
		}
	}
	sort.Slice(logs, func(i, j int) bool {
		a, b := logs[i], logs[j]
		return a.BlockNumber < b.BlockNumber || (a.BlockNumber == b.BlockNumber && a.LogIndex < b.LogIndex)
	})
	m.logger.WithFields(logrus.Fields{
		"count":      len(logs),
		"from_block": blocksRange.From,
		"to_block":   blocksRange.To,
	}).Info("fetched logs in range")
	if len(logs) > 0 {
		err = m.repo.Logs.Ensure(ctx, logs...)
		if err != nil {
			return err
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

	m.submitLogs(logs, blocksRange.To)
	return nil
}

func (m *ContractMonitor) submitLogs(logs []*entity.Log, endBlock uint) {
	logBatches := SplitLogsInBatches(logs)
	m.logger.WithFields(logrus.Fields{
		"count": len(logs),
		"jobs":  len(logBatches),
	}).Info("create jobs for logs processor")
	for _, batch := range logBatches {
		m.logger.WithFields(logrus.Fields{
			"count":        len(batch.Logs),
			"block_number": batch.BlockNumber,
		}).Debug("submitting logs batch to logs processor")
		m.logsChan <- batch
	}
	if len(logBatches) == 0 || logBatches[len(logBatches)-1].BlockNumber < endBlock {
		m.logsChan <- &LogsBatch{
			BlockNumber: endBlock,
			Logs:        nil,
		}
	}
}

func (m *ContractMonitor) StartLogsProcessor(ctx context.Context) {
	m.logger.Info("starting logs processor")
	for {
		select {
		case <-ctx.Done():
			return
		case logs := <-m.logsChan:
			if logs == nil {
				continue
			}
			m.processLogsBatch(ctx, logs)
		}
	}
}

func (m *ContractMonitor) processLogsBatch(ctx context.Context, logs *LogsBatch) {
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
				if utils.ContextSleep(ctx, time.Second) == nil {
					return
				}
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
				if utils.ContextSleep(ctx, time.Second) == nil {
					return
				}
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

func (m *ContractMonitor) tryToGetBlockTimestamp(ctx context.Context, blockNumber uint) error {
	_, err := m.repo.BlockTimestamps.GetByBlockNumber(ctx, m.cfg.Chain.ChainID, blockNumber)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			m.logger.WithField("block_number", blockNumber).Debug("fetching block timestamp")
			header, err := m.client.HeaderByNumber(ctx, blockNumber)
			if err != nil {
				return fmt.Errorf("can't request block header: %w", err)
			}
			return m.repo.BlockTimestamps.Ensure(ctx, &entity.BlockTimestamp{
				ChainID:     m.cfg.Chain.ChainID,
				BlockNumber: blockNumber,
				Timestamp:   time.Unix(int64(header.Time), 0),
			})
		}
		return fmt.Errorf("can't get block timestamp from db: %w", err)
	}
	m.logger.WithField("block_number", blockNumber).Debug("timestamp already exists, skipping")
	return nil
}

func (m *ContractMonitor) tryToProcessLogsBatch(ctx context.Context, batch *LogsBatch) error {
	m.logger.WithFields(logrus.Fields{
		"count":        len(batch.Logs),
		"block_number": batch.BlockNumber,
	}).Debug("processing logs batch")
	for _, log := range batch.Logs {
		event, data, err := m.contract.ABI.ParseLog(log)
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

func (m *ContractMonitor) recordHeadBlockNumber(blockNumber uint) {
	if blockNumber < m.headBlock {
		return
	}

	m.headBlock = blockNumber
	m.headBlockMetric.Set(float64(blockNumber))
	m.recordIsSynced()
}

func (m *ContractMonitor) recordIsSynced() {
	m.isSynced = m.logsCursor.LastProcessedBlock+defaultSyncedThreshold > m.headBlock
	if m.isSynced {
		m.syncedMetric.Set(1)
	} else {
		m.syncedMetric.Set(0)
	}
}

func (m *ContractMonitor) recordFetchedBlockNumber(ctx context.Context, blockNumber uint) error {
	if blockNumber < m.logsCursor.LastFetchedBlock {
		return nil
	}

	m.logsCursor.LastFetchedBlock = blockNumber
	m.fetchedBlockMetric.Set(float64(blockNumber))
	err := m.repo.LogsCursors.Ensure(ctx, m.logsCursor)
	if err != nil {
		return err
	}
	return nil
}

func (m *ContractMonitor) recordProcessedBlockNumber(ctx context.Context, blockNumber uint) error {
	if blockNumber < m.logsCursor.LastProcessedBlock {
		return nil
	}

	m.logsCursor.LastProcessedBlock = blockNumber
	m.processedBlockMetric.Set(float64(blockNumber))
	m.recordIsSynced()
	err := m.repo.LogsCursors.Ensure(ctx, m.logsCursor)
	if err != nil {
		return err
	}
	return nil
}
