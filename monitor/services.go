package monitor

import (
	"amb-monitor/db"
	"amb-monitor/ethclient"
	"amb-monitor/logging"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v4"
	"math/big"
	"sort"
	"sync"
	"time"
)

func (state *BridgeSideMonitor) StartBlockWatcher(ctx context.Context) {
	logger := logging.GetLogger(state.Side + "-block-number-fetcher")
	t := time.NewTicker(time.Duration(state.BlockTime) * time.Millisecond)
	cur := state.CurrentBlock

	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			n, err := state.Client.BlockNumber()
			if err != nil {
				fmt.Println(err)
				continue
			}
			logger.Printf("fetched block number %d\n", n)
			n -= state.RequiredBlockConfirmations
			if n >= cur {
				maxRangeSize := uint64(10000)
				for cur+maxRangeSize < n {
					logger.Printf("push range (%d, %d)\n", cur, cur+maxRangeSize)
					state.BlockRangeQueue <- &BlockRange{cur, cur + maxRangeSize, true}
					cur += maxRangeSize + 1
				}
				logger.Printf("push range (%d, %d)\n", cur, n)
				state.BlockRangeQueue <- &BlockRange{cur, n, true}
				cur = n + 1
			}
		}
	}
}

func (state *BridgeSideMonitor) StartEventFetcher(ctx context.Context) {
	logger := logging.GetLogger(state.Side + "-event-fetcher")
	for {
		select {
		case <-ctx.Done():
			return
		case blockRange := <-state.BlockRangeQueue:
			for {
				logs, err := state.Client.FilterLogs(ethereum.FilterQuery{
					FromBlock: big.NewInt(int64(blockRange.From)),
					ToBlock:   big.NewInt(int64(blockRange.To)),
					Addresses: []common.Address{state.BridgeContract.Address},
				})
				if err != nil {
					fmt.Println(err)
					continue
				}
				logger.Printf("Fetched %d logs from range (%d, %d)\n", len(logs), blockRange.From, blockRange.To)
				if len(logs) == 0 {
					break
				}
				sort.Slice(logs, func(i, j int) bool {
					a, b := &logs[i], &logs[j]
					return a.BlockNumber < b.BlockNumber || (a.BlockNumber == b.BlockNumber && a.Index < b.Index)
				})
				batchStartIndex := 0
				for i, log := range logs {
					if log.BlockNumber > logs[batchStartIndex].BlockNumber {
						state.LogBatchQueue <- &LogBatch{
							BlockNumber: logs[batchStartIndex].BlockNumber,
							Logs:        logs[batchStartIndex:i],
							CommitBlock: blockRange.CommitBlock,
						}
						batchStartIndex = i
					}
				}
				state.LogBatchQueue <- &LogBatch{
					BlockNumber: logs[batchStartIndex].BlockNumber,
					Logs:        logs[batchStartIndex:],
					CommitBlock: blockRange.CommitBlock,
				}
				break
			}
		}
	}
}

func (state *BridgeSideMonitor) StartLogDispatcher(ctx context.Context, conn *db.DB) {
	logger := logging.GetLogger(state.Side + "-log-processor")
	for {
		select {
		case <-ctx.Done():
			return
		case logBatch := <-state.LogBatchQueue:
			for {
				actions := make(db.ExecBatch, 0, len(logBatch.Logs)+1)
				logger.Printf("processing %d events from block %d\n", len(logBatch.Logs), logBatch.BlockNumber)
				for _, log := range logBatch.Logs {
					eventName, event, err := state.BridgeContract.ParseLog(&log)
					if err != nil {
						logger.Fatalln("log parsing has failed", err)
					}
					if event == nil {
						logger.Printf("unknown event with id %s, skipping\n", log.Topics[0].Hex())
						continue
					}
					if !state.HasEventHandler(eventName) {
						logger.Printf("no handler for event %s\n", eventName)
						continue
					}
					handler, err := state.HandleEvent(eventName, log, event)
					if err != nil {
						logger.Fatalln("handling of event ", eventName, " has failed: ", err)
					}
					actions = append(actions, handler)
				}
				if logBatch.CommitBlock {
					actions = append(actions, state.InsertNewBlock(logBatch.BlockNumber))
					actions = append(actions, state.UpdateCurrentBlockNumber(logBatch.BlockNumber+1))
				}
				if len(actions) > 0 {
					state.DbTxQueue <- actions
				}
				break
			}
		}
	}
}

func (state *BridgeSideMonitor) StartDbWriter(ctx context.Context, conn *db.DB) {
	logger := logging.GetLogger(state.Side + "-db-writer")

	for {
		select {
		case <-ctx.Done():
			return
		case actions := <-state.DbTxQueue:
			for {
				err := actions.ApplyTo(conn)
				if err != nil {
					if err.Error() == "closed pool" {
						return
					}
					logger.Println(err)
					continue
				}
				break
			}
		}
	}
}

func (state *BridgeMonitor) StartReindexer(ctx context.Context, conn *db.DB) {
	logger := logging.GetLogger("db-reindexer")
	t := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			err := state.Reindex(conn)
			if err != nil {
				if err.Error() == "closed pool" {
					return
				}
				logger.Println(err)
			}
		}
	}
}

func StartBlockIndexer(ctx context.Context, conn *db.DB, clients map[string]*ethclient.Client) {
	logger := logging.GetLogger("block-indexer")

	t := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			logger.Println("searching for a batch of blocks without timestamps")
			rows, err := conn.Query(ctx, "SELECT id, chain_id::text, block_number FROM block WHERE timestamp IS NULL LIMIT 100")
			if err != nil {
				logger.Println(err)
				rows.Close()
				continue
			}

			var wg sync.WaitGroup
			var batch pgx.Batch
			var id, blockNumber uint64
			var chainId string

			for rows.Next() {
				err := rows.Scan(&id, &chainId, &blockNumber)
				if err != nil {
					logger.Println(err)
					continue
				}

				client, ok := clients[chainId]
				if !ok {
					panic("required eth client does not exist")
				}

				wg.Add(1)
				go func(id uint64, blockNumber uint64, chainId string) {
					defer wg.Done()

					header, err := client.HeaderByNumber(blockNumber)
					if err != nil {
						logger.Println(err)
					} else {
						batch.Queue("UPDATE block SET timestamp=$2 WHERE id=$1", id, time.Unix(int64(header.Time), 0))
					}
				}(id, blockNumber, chainId)
			}
			if rows.Err() != nil {
				logger.Println(rows.Err())
				continue
			}
			rows.Close()

			if batch.Len() > 0 {
				wg.Wait()
				err = conn.SendBatch(context.Background(), &batch).Close()
				if err != nil {
					logger.Println(err)
				}
			}
		}
	}
}
