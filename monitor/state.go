package monitor

import (
	"amb-monitor/config"
	"amb-monitor/contract"
	"amb-monitor/contract/constants"
	"amb-monitor/db"
	"amb-monitor/ethclient"
	"amb-monitor/logging"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgtype/pgxtype"
	"github.com/jackc/pgx/v4"
)

type BlockRange struct {
	From        uint64
	To          uint64
	CommitBlock bool
}

type LogBatch struct {
	BlockNumber uint64
	Logs        []types.Log
	CommitBlock bool
}

type BridgeSideMonitor struct {
	parent                     *BridgeMonitor
	Side                       string
	dbId                       uint64
	ChainId                    string
	RequiredBlockConfirmations uint64
	BlockTime                  uint64
	StartBlock                 uint64
	CurrentBlock               uint64
	BridgeContract             *contract.Contract
	Client                     *ethclient.Client
	BlockRangeQueue            chan *BlockRange
	LogBatchQueue              chan *LogBatch
	DbTxQueue                  chan db.Executable
	eventHandlers              map[string]EventHandler
}

type BridgeMonitor struct {
	Id      string
	Home    *BridgeSideMonitor
	Foreign *BridgeSideMonitor
}

type Monitor struct {
	DbTxPool chan db.Executable
}

func NewSideState(cfg *config.BridgeSideConfig, conn *db.DB) (*BridgeSideMonitor, error) {
	client, err := ethclient.NewClient(cfg.Chain.RPC.Host, cfg.Chain.RPC.Timeout)
	if err != nil {
		return nil, err
	}
	chainId, err := client.ChainID()
	if err != nil {
		return nil, err
	}
	bridge := contract.NewContract(client, cfg.Address, constants.AMB)
	res, err := bridge.Call("requiredBlockConfirmations")
	if err != nil {
		return nil, err
	}
	if n, ok := res[0].(uint64); ok {
		state := &BridgeSideMonitor{
			ChainId:                    chainId,
			RequiredBlockConfirmations: n,
			StartBlock:                 cfg.StartBlock,
			CurrentBlock:               cfg.StartBlock,
			BlockTime:                  cfg.Chain.BlockTime,
			BridgeContract:             bridge,
			Client:                     client,
			BlockRangeQueue:            make(chan *BlockRange, 3),
			LogBatchQueue:              make(chan *LogBatch, 10),
			DbTxQueue:                  make(chan db.Executable, 10),
			eventHandlers:              make(map[string]EventHandler, 10),
		}
		go func() {
			for _, x := range cfg.ManualBlockRanges {
				state.BlockRangeQueue <- &BlockRange{x[0], x[1], false}
			}
		}()

		err = db.ExecFuncAtomic(state.ReadFromDB).ApplyTo(conn)
		if err == pgx.ErrNoRows {
			err = db.ExecFuncAtomic(state.Insert).ApplyTo(conn)
		}
		if err != nil {
			return nil, err
		}
		return state, nil
	}
	return nil, fmt.Errorf("not uint64")
}

func (state *BridgeSideMonitor) ReadFromDB(q pgxtype.Querier) error {
	row := q.QueryRow(
		context.Background(),
		"SELECT id, start_block, current_block FROM bridge WHERE chain_id = $1 AND address = $2",
		state.ChainId,
		state.BridgeContract.Address.Hex(),
	)
	return row.Scan(&state.dbId, &state.StartBlock, &state.CurrentBlock)
}

func (state *BridgeSideMonitor) Insert(q pgxtype.Querier) error {
	row := q.QueryRow(
		context.Background(),
		"INSERT INTO bridge (chain_id, address, start_block, current_block) VALUES ($1, $2, $3, $4) RETURNING id",
		state.ChainId, state.BridgeContract.Address.Hex(), state.StartBlock, state.CurrentBlock,
	)
	return row.Scan(&state.dbId)
}

func (state *BridgeSideMonitor) Save(q pgxtype.Querier) error {
	_, err := q.Exec(
		context.Background(),
		"UPDATE bridge SET current_block = $2 WHERE id = $1",
		state.dbId, state.CurrentBlock,
	)
	return err
}

func (state *BridgeMonitor) Insert(q pgxtype.Querier) error {
	_, err := q.Exec(
		context.Background(),
		"INSERT INTO bridge_pair (id, home_bridge, foreign_bridge) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		state.Id, state.Home.dbId, state.Foreign.dbId,
	)
	return err
}

func NewState(cfg *config.BridgeConfig, conn *db.DB) (*BridgeMonitor, error) {
	home, err := NewSideState(cfg.Home, conn)
	if err != nil {
		return nil, err
	}
	foreign, err := NewSideState(cfg.Foreign, conn)
	if err != nil {
		return nil, err
	}
	home.Side = "home"
	foreign.Side = "foreign"

	state := &BridgeMonitor{
		Id:      cfg.Id,
		Home:    home,
		Foreign: foreign,
	}
	home.parent = state
	foreign.parent = state

	home.RegisterHandler("UserRequestForSignature", HandleUserRequestForSignature)
	home.RegisterHandler("UserRequestForSignature0", HandleLegacyUserRequestForSignature)
	home.RegisterHandler("SignedForUserRequest", HandleSignedForUserRequest)
	home.RegisterHandler("SignedForAffirmation", HandleSignedForAffirmation)
	home.RegisterHandler("AffirmationCompleted", HandleAffirmationCompleted)
	foreign.RegisterHandler("UserRequestForAffirmation", HandleUserRequestForAffirmation)
	foreign.RegisterHandler("UserRequestForAffirmation0", HandleLegacyUserRequestForAffirmation)
	foreign.RegisterHandler("RelayedMessage", HandleRelayedMessage)

	err = db.ExecFuncAtomic(state.Insert).ApplyTo(conn)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (state *BridgeSideMonitor) HasEventHandler(name string) bool {
	_, ok := state.eventHandlers[name]
	return ok
}

func (state *BridgeSideMonitor) HandleEvent(name string, log types.Log, event map[string]interface{}) (db.Executable, error) {
	handler, ok := state.eventHandlers[name]
	if !ok {
		panic("no event handler")
	}
	return handler(state, log, event)
}

func (state *BridgeSideMonitor) RegisterHandler(name string, handler EventHandler) {
	if !state.BridgeContract.HasEvent(name) {
		panic("underlying contract does not have such event")
	}
	_, ok := state.eventHandlers[name]
	if ok {
		panic("cannot register duplicated handler")
	}
	state.eventHandlers[name] = handler
}

func (state *BridgeSideMonitor) InsertNewBlock(n uint64) db.Executable {
	return db.ExecFuncAtomic(func(q pgxtype.Querier) error {
		_, err := q.Exec(
			context.Background(),
			"INSERT INTO block (chain_id, block_number)"+
				"VALUES ($1, $2)"+
				"ON CONFLICT (chain_id, block_number) DO NOTHING",
			state.ChainId, n,
		)
		return err
	})
}

func (state *BridgeSideMonitor) UpdateCurrentBlockNumber(n uint64) db.Executable {
	return db.ExecFuncAtomic(func(q pgxtype.Querier) error {
		_, err := q.Exec(
			context.Background(),
			"UPDATE bridge SET current_block=$1 WHERE id=$2",
			n, state.dbId,
		)
		return err
	})
}

func (state *BridgeMonitor) Reindex(conn *db.DB) error {
	logger := logging.GetLogger("reindex")
	return db.ExecBatch{
		db.ExecFunc(func(q pgxtype.Querier) error {
			tag, err := q.Exec(
				context.Background(),
				"UPDATE message_confirmation mc "+
					"SET msg_id = m.id, tmp_msg_hash = NULL, tmp_bridge_id = NULL "+
					"FROM message m "+
					"WHERE mc.tmp_bridge_id = $1 AND mc.tmp_msg_hash = m.msg_hash",
				state.Id,
			)
			if err != nil {
				return err
			}
			logger.Printf("reindexed and updated %d message_confirmation records\n", tag.RowsAffected())
			return nil
		}),
		db.ExecFunc(func(q pgxtype.Querier) error {
			tag, err := q.Exec(
				context.Background(),
				"UPDATE message_execution me "+
					"SET msg_id = m.id, tmp_message_id = NULL, tmp_bridge_id = NULL "+
					"FROM message m "+
					"WHERE me.tmp_bridge_id = $1 AND me.tmp_message_id = m.message_id",
				state.Id,
			)
			if err != nil {
				return err
			}
			logger.Printf("reindexed and updated %d message_execution records\n", tag.RowsAffected())
			return nil
		}),
	}.ApplyTo(conn)
}
