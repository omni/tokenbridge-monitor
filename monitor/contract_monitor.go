package monitor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"
	"tokenbridge-monitor/config"
	"tokenbridge-monitor/contract"
	"tokenbridge-monitor/contract/abi"
	"tokenbridge-monitor/entity"
	"tokenbridge-monitor/ethclient"
	"tokenbridge-monitor/logging"
	"tokenbridge-monitor/repository"
	"tokenbridge-monitor/utils"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const defaultSyncedThreshold = 10
const defaultBlockRangesChanCap = 10
const defaultLogsChanCap = 200
const defaultEventHandlersMapCap = 20

type ContractMonitor struct {
	bridgeCfg            *config.BridgeConfig
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
	isSynced             bool
	syncedMetric         prometheus.Gauge
	headBlockMetric      prometheus.Gauge
	fetchedBlockMetric   prometheus.Gauge
	processedBlockMetric prometheus.Gauge
}

func NewContractMonitor(ctx context.Context, logger logging.Logger, repo *repository.Repo, bridgeCfg *config.BridgeConfig, cfg *config.BridgeSideConfig) (*ContractMonitor, error) {
	client, err := ethclient.NewClient(cfg.Chain.RPC.Host, cfg.Chain.RPC.Timeout, cfg.Chain.ChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to start eth client: %w", err)
	}
	contractAbi := abi.AMB
	if bridgeCfg.IsErcToNative {
		contractAbi = abi.ERC_TO_NATIVE
	}
	bridgeContract := contract.NewContract(client, cfg.Address, contractAbi)
	if cfg.ValidatorContractAddress == (common.Address{}) {
		cfg.ValidatorContractAddress, err = bridgeContract.ValidatorContractAddress(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot get validator contract address: %w", err)
		}
		logger.WithFields(logrus.Fields{
			"chain_id":                   client.ChainID,
			"bridge_address":             cfg.Address,
			"validator_contract_address": cfg.ValidatorContractAddress,
			"start_block":                cfg.StartBlock,
		}).Info("obtained validator contract address")
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
			return nil, fmt.Errorf("failed to read logs cursor: %w", err)
		}
	}
	commonLabels := prometheus.Labels{
		"bridge_id": bridgeCfg.ID,
		"chain_id":  client.ChainID,
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
	events := m.contract.AllEvents()
	for e := range m.eventHandlers {
		if !events[e] {
			return fmt.Errorf("contract does not have %s event in its ABI", e)
		}
	}
	return nil
}

func (m *ContractMonitor) Start(ctx context.Context) {
	lastProcessedBlock := m.logsCursor.LastProcessedBlock
	lastFetchedBlock := m.logsCursor.LastFetchedBlock
	go m.StartBlockFetcher(ctx, lastFetchedBlock+1)
	go m.StartLogsProcessor(ctx)
	m.LoadUnprocessedLogs(ctx, lastProcessedBlock+1, lastFetchedBlock)
	go m.StartLogsFetcher(ctx)
}

func (m *ContractMonitor) LoadUnprocessedLogs(ctx context.Context, fromBlock, toBlock uint) {
	m.logger.WithFields(logrus.Fields{
		"from_block": fromBlock,
		"to_block":   toBlock,
	}).Info("loading fetched but not yet processed blocks")

	addresses := m.cfg.ContractAddresses(fromBlock, toBlock)
	for {
		logs, err := m.repo.Logs.FindByBlockRange(ctx, m.client.ChainID, addresses, fromBlock, toBlock)
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

	if len(m.cfg.RefetchEvents) > 0 {
		m.RefetchEvents(start - 1)
	}

	for {
		head, err := m.client.BlockNumber(ctx)
		if err != nil {
			m.logger.WithError(err).Error("can't fetch latest block number")
		} else {
			head -= m.cfg.BlockConfirmations
			m.recordHeadBlockNumber(head)

			for start <= head {
				end := start + m.cfg.MaxBlockRangeSize - 1
				if end > head {
					end = head
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

func (m *ContractMonitor) RefetchEvents(lastFetchedBlock uint) {
	m.logger.Info("refetching old events")
	for _, job := range m.cfg.RefetchEvents {
		fromBlock := job.StartBlock
		if fromBlock < m.cfg.StartBlock {
			fromBlock = m.cfg.StartBlock
		}
		toBlock := job.EndBlock
		if toBlock == 0 || toBlock > lastFetchedBlock {
			toBlock = lastFetchedBlock
		}

		for fromBlock <= toBlock {
			end := fromBlock + m.cfg.MaxBlockRangeSize - 1
			if end > toBlock {
				end = toBlock
			}
			m.logger.WithFields(logrus.Fields{
				"from_block": fromBlock,
				"to_block":   end,
			}).Info("scheduling new block range logs search")
			br := &BlocksRange{
				From: fromBlock,
				To:   end,
			}
			if job.Event != "" {
				topic := crypto.Keccak256Hash([]byte(job.Event))
				br.Topic = &topic
			}
			m.blocksRangeChan <- br
			fromBlock = end + 1
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
	var qs []ethereum.FilterQuery
	q := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(blocksRange.From)),
		ToBlock:   big.NewInt(int64(blocksRange.To)),
		Addresses: []common.Address{m.cfg.Address, m.cfg.ValidatorContractAddress},
	}
	if blocksRange.Topic != nil {
		q.Topics = [][]common.Hash{{*blocksRange.Topic}}
	}
	qs = append(qs, q)
	if m.bridgeCfg.IsErcToNative {
		for _, token := range m.cfg.ErcToNativeTokens {
			if token.StartBlock > 0 && blocksRange.To < token.StartBlock {
				continue
			}
			if token.EndBlock > 0 && blocksRange.From > token.EndBlock {
				continue
			}
			qc := q
			if blocksRange.Topic != nil {
				qc.Topics = [][]common.Hash{{*blocksRange.Topic}, {}, {m.cfg.Address.Hash()}}
			} else {
				qc.Topics = [][]common.Hash{{}, {}, {m.cfg.Address.Hash()}}
			}
			qc.Addresses = []common.Address{token.Address}
			if token.StartBlock > 0 && token.StartBlock > blocksRange.From {
				qc.FromBlock = big.NewInt(int64(token.StartBlock))
			}
			if token.EndBlock > 0 && token.EndBlock < blocksRange.To {
				qc.ToBlock = big.NewInt(int64(token.EndBlock))
			}
			qs = append(qs, qc)
		}
	}
	return qs
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
		return fmt.Errorf("can't get block timestamp from db: %w", err)
	}
	if ts != nil {
		m.logger.WithField("block_number", blockNumber).Debug("timestamp already exists, skipping")
		return nil
	}
	m.logger.WithField("block_number", blockNumber).Debug("fetching block timestamp")
	header, err := m.client.HeaderByNumber(ctx, blockNumber)
	if err != nil {
		return fmt.Errorf("can't request block header: %w", err)
	}
	return m.repo.BlockTimestamps.Ensure(ctx, &entity.BlockTimestamp{
		ChainID:     m.client.ChainID,
		BlockNumber: blockNumber,
		Timestamp:   time.Unix(int64(header.Time), 0),
	})
}

func (m *ContractMonitor) tryToProcessLogsBatch(ctx context.Context, batch *LogsBatch) error {
	m.logger.WithFields(logrus.Fields{
		"count":        len(batch.Logs),
		"block_number": batch.BlockNumber,
	}).Debug("processing logs batch")
	for _, log := range batch.Logs {
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
