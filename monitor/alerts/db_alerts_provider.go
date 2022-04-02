package alerts

import (
	"amb-monitor/db"
	"context"
	"fmt"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/ethereum/go-ethereum/common"
	"github.com/lib/pq"
)

type DBAlertsProvider struct {
	db *db.DB
}

func NewDBAlertsProvider(db *db.DB) *DBAlertsProvider {
	return &DBAlertsProvider{
		db: db,
	}
}

type UnknownConfirmation struct {
	ChainID         string         `db:"chain_id"`
	BlockNumber     uint64         `db:"block_number"`
	Age             time.Duration  `db:"age"`
	TransactionHash common.Hash    `db:"transaction_hash"`
	Signer          common.Address `db:"signer"`
	MsgHash         common.Hash    `db:"msg_hash"`
}

func (c *UnknownConfirmation) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"signer":       c.Signer.String(),
			"msg_hash":     c.MsgHash.String(),
		},
		Value: float64(c.Age),
	}
}

func (p *DBAlertsProvider) findMinProcessedTime(ctx context.Context, params *AlertJobParams) (*time.Time, error) {
	q, args, err := sq.Select("MIN(lcb.timestamp)").
		From("logs_cursors lc").
		Where(sq.Or{
			sq.And{
				sq.Eq{"lc.chain_id": params.HomeChainID},
				sq.GtOrEq{"lc.address": params.HomeBridgeAddress},
			},
			sq.And{
				sq.Eq{"lc.chain_id": params.ForeignChainID},
				sq.GtOrEq{"lc.address": params.ForeignBridgeAddress},
			},
		}).
		Join("block_timestamps lcb ON lc.chain_id = lcb.chain_id AND lcb.block_number = lc.last_processed_block").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := new(time.Time)
	err = p.db.GetContext(ctx, res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select last processed timestamp: %w", err)
	}
	return res, nil
}

func (p *DBAlertsProvider) FindUnknownConfirmations(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	minProcessedTS, err := p.findMinProcessedTime(ctx, params)
	if err != nil {
		return nil, err
	}
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "sm.signer", "sm.msg_hash", "EXTRACT(EPOCH FROM now() - bt.timestamp)::int as age").
		From("signed_messages sm").
		Join("logs l ON l.id = sm.log_id").
		Join("block_timestamps bt on bt.chain_id = l.chain_id AND bt.block_number = l.block_number").
		LeftJoin("messages m ON sm.bridge_id = m.bridge_id AND m.msg_hash = sm.msg_hash").
		Where(sq.Eq{"m.id": nil, "sm.bridge_id": params.Bridge, "l.chain_id": params.HomeChainID}).
		Where(sq.GtOrEq{"l.block_number": params.HomeStartBlockNumber}).
		Where(sq.LtOrEq{"bt.timestamp": minProcessedTS}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := make([]UnknownConfirmation, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type UnknownExecution struct {
	ChainID         string        `db:"chain_id"`
	BlockNumber     uint64        `db:"block_number"`
	Age             time.Duration `db:"age"`
	TransactionHash common.Hash   `db:"transaction_hash"`
	MessageID       common.Hash   `db:"message_id"`
}

func (c *UnknownExecution) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"message_id":   c.MessageID.String(),
		},
		Value: float64(c.Age),
	}
}

func (p *DBAlertsProvider) FindUnknownExecutions(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	minProcessedTS, err := p.findMinProcessedTime(ctx, params)
	if err != nil {
		return nil, err
	}
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "em.message_id", "EXTRACT(EPOCH FROM now() - bt.timestamp)::int as age").
		From("executed_messages em").
		Join("logs l ON l.id = em.log_id").
		Join("block_timestamps bt on bt.chain_id = l.chain_id AND bt.block_number = l.block_number").
		LeftJoin("messages m ON em.bridge_id = m.bridge_id AND em.message_id = m.message_id").
		Where(sq.Eq{"m.id": nil, "em.bridge_id": params.Bridge}).
		Where(sq.Or{
			sq.And{
				sq.Eq{"l.chain_id": params.HomeChainID},
				sq.GtOrEq{"l.block_number": params.HomeStartBlockNumber},
			},
			sq.And{
				sq.Eq{"l.chain_id": params.ForeignChainID},
				sq.GtOrEq{"l.block_number": params.ForeignStartBlockNumber},
			},
		}).
		Where(sq.LtOrEq{"bt.timestamp": minProcessedTS}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := make([]UnknownExecution, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type StuckMessage struct {
	ChainID         string        `db:"chain_id"`
	BlockNumber     uint64        `db:"block_number"`
	Age             time.Duration `db:"age"`
	TransactionHash common.Hash   `db:"transaction_hash"`
	MsgHash         common.Hash   `db:"msg_hash"`
	Count           uint64        `db:"count"`
}

func (c *StuckMessage) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"msg_hash":     c.MsgHash.String(),
			"count":        strconv.FormatUint(c.Count, 10),
		},
		Value: float64(c.Age),
	}
}

func (p *DBAlertsProvider) FindStuckMessages(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	query := `
		SELECT l.chain_id,
		       l.block_number,
		       l.transaction_hash,
		       sm.msg_hash,
		       count(s.log_id) as count,
		       EXTRACT(EPOCH FROM now() - ts.timestamp)::int as age
		FROM sent_messages sm
		         JOIN logs l on l.id = sm.log_id
		         JOIN block_timestamps ts on ts.chain_id = l.chain_id AND ts.block_number = l.block_number
		         JOIN messages m on sm.bridge_id = m.bridge_id AND m.msg_hash = sm.msg_hash
		         LEFT JOIN signed_messages s on s.bridge_id = m.bridge_id AND m.msg_hash = s.msg_hash
		         LEFT JOIN collected_messages cm on m.bridge_id = cm.bridge_id AND cm.msg_hash = m.msg_hash
		         LEFT JOIN executed_messages em on m.bridge_id = em.bridge_id AND em.message_id = m.message_id
		WHERE m.direction::direction_enum = 'home_to_foreign'
		  AND (
		    cm.log_id IS NULL OR
		    (em.log_id IS NULL AND m.data_type = 0 AND m.sender = ANY($4))
		  )
		  AND sm.bridge_id = $1
		  AND l.block_number >= $2
		GROUP BY sm.log_id, l.id, ts.timestamp
		UNION
		SELECT l.chain_id,
		       l.block_number,
		       l.transaction_hash,
		       sm.msg_hash,
		       count(s.log_id) as count,
		       EXTRACT(EPOCH FROM now() - ts.timestamp)::int as age
		FROM sent_messages sm
		         JOIN logs l on l.id = sm.log_id
		         JOIN block_timestamps ts on ts.chain_id = l.chain_id AND ts.block_number = l.block_number
		         JOIN messages m on sm.bridge_id = m.bridge_id AND m.msg_hash = sm.msg_hash
		         LEFT JOIN signed_messages s on s.bridge_id = m.bridge_id AND m.msg_hash = s.msg_hash
		         LEFT JOIN executed_messages em on m.bridge_id = em.bridge_id AND em.message_id = m.message_id
		WHERE m.direction::direction_enum = 'foreign_to_home'
		  AND em.log_id IS NULL
		  AND sm.bridge_id = $1
		  AND l.block_number >= $3
		GROUP BY sm.log_id, l.id, ts.timestamp`
	res := make([]StuckMessage, 0, 5)
	var whitelisted pq.ByteaArray
	for _, addr := range params.HomeWhitelistedSenders {
		whitelisted = append(whitelisted, addr.Bytes())
	}
	err := p.db.SelectContext(ctx, &res, query, params.Bridge, params.HomeStartBlockNumber, params.ForeignStartBlockNumber, whitelisted)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type FailedExecution struct {
	ChainID         string         `db:"chain_id"`
	BlockNumber     uint64         `db:"block_number"`
	Age             time.Duration  `db:"age"`
	TransactionHash common.Hash    `db:"transaction_hash"`
	Sender          common.Address `db:"sender"`
	Executor        common.Address `db:"executor"`
}

func (c *FailedExecution) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"sender":       c.Sender.String(),
			"executor":     c.Executor.String(),
		},
		Value: float64(c.Age),
	}
}

func (p *DBAlertsProvider) FindFailedExecutions(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "m.sender", "m.executor", "EXTRACT(EPOCH FROM now() - bt.timestamp)::int as age").
		From("messages m").
		Join("executed_messages em on m.bridge_id = em.bridge_id AND em.message_id = m.message_id").
		Join("logs l ON l.id = em.log_id").
		Join("block_timestamps bt on bt.chain_id = l.chain_id AND bt.block_number = l.block_number").
		Where(sq.Eq{"em.status": false, "m.data_type": []int{0, 128}, "em.bridge_id": params.Bridge}).
		Where(sq.Or{
			sq.And{
				sq.Eq{"l.chain_id": params.HomeChainID},
				sq.GtOrEq{"l.block_number": params.HomeStartBlockNumber},
			},
			sq.And{
				sq.Eq{"l.chain_id": params.ForeignChainID},
				sq.GtOrEq{"l.block_number": params.ForeignStartBlockNumber},
			},
		}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := make([]FailedExecution, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type StuckInformationRequest struct {
	ChainID         string        `db:"chain_id"`
	BlockNumber     uint64        `db:"block_number"`
	Age             time.Duration `db:"age"`
	TransactionHash common.Hash   `db:"transaction_hash"`
	MessageID       common.Hash   `db:"message_id"`
	Count           uint64        `db:"count"`
}

func (c *StuckInformationRequest) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"message_id":   c.MessageID.String(),
			"count":        strconv.FormatUint(c.Count, 10),
		},
		Value: float64(c.Age),
	}
}

func (p *DBAlertsProvider) FindStuckInformationRequests(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "r.message_id", "count(s.log_id) as count", "EXTRACT(EPOCH FROM now() - bt.timestamp)::int as age").
		From("sent_information_requests sr").
		Join("logs l on l.id = sr.log_id").
		Join("block_timestamps bt on bt.chain_id = l.chain_id AND bt.block_number = l.block_number").
		Join("information_requests r on sr.bridge_id = r.bridge_id AND r.message_id = sr.message_id").
		LeftJoin("signed_information_requests s on s.bridge_id = r.bridge_id AND r.message_id = s.message_id").
		LeftJoin("executed_information_requests er on r.bridge_id = er.bridge_id AND r.message_id = er.message_id").
		GroupBy("l.id", "r.id", "bt.timestamp").
		Where(sq.GtOrEq{
			"l.block_number": params.HomeStartBlockNumber,
		}).
		Where(sq.Eq{
			"l.chain_id":   params.HomeChainID,
			"er.log_id":    nil,
			"sr.bridge_id": params.Bridge,
		}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}

	res := make([]StuckInformationRequest, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type FailedInformationRequest struct {
	ChainID         string         `db:"chain_id"`
	BlockNumber     uint64         `db:"block_number"`
	Age             time.Duration  `db:"age"`
	TransactionHash common.Hash    `db:"transaction_hash"`
	Sender          common.Address `db:"sender"`
	Executor        common.Address `db:"executor"`
	Status          bool           `db:"status"`
	CallbackStatus  bool           `db:"callback_status"`
}

func (c *FailedInformationRequest) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":        c.ChainID,
			"block_number":    strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":         c.TransactionHash.String(),
			"sender":          c.Sender.String(),
			"executor":        c.Executor.String(),
			"status":          strconv.FormatBool(c.Status),
			"callback_status": strconv.FormatBool(c.CallbackStatus),
		},
		Value: float64(c.Age),
	}
}

func (p *DBAlertsProvider) FindFailedInformationRequests(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "r.sender", "r.executor", "er.status", "er.callback_status", "EXTRACT(EPOCH FROM now() - bt.timestamp)::int as age").
		From("information_requests r").
		Join("executed_information_requests er on r.bridge_id = er.bridge_id AND er.message_id = r.message_id").
		Join("logs l ON l.id = er.log_id").
		Join("block_timestamps bt on bt.chain_id = l.chain_id AND bt.block_number = l.block_number").
		Where(sq.Or{
			sq.Eq{"er.status": false},
			sq.Eq{"er.callback_status": false},
		}).
		Where(sq.Eq{"er.bridge_id": params.Bridge}).
		Where(sq.And{
			sq.Eq{"l.chain_id": params.HomeChainID},
			sq.GtOrEq{"l.block_number": params.HomeStartBlockNumber},
		}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := make([]FailedInformationRequest, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type DifferentInformationSignature struct {
	ChainID         string        `db:"chain_id"`
	BlockNumber     uint64        `db:"block_number"`
	Age             time.Duration `db:"age"`
	TransactionHash common.Hash   `db:"transaction_hash"`
	MessageID       common.Hash   `db:"message_id"`
	Count           uint64        `db:"count"`
}

func (c *DifferentInformationSignature) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"message_id":   c.MessageID.String(),
			"count":        strconv.FormatUint(c.Count, 10),
		},
		Value: float64(c.Age),
	}
}

func (p *DBAlertsProvider) FindDifferentInformationSignatures(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "r.message_id", "count(DISTINCT s.data) as count", "EXTRACT(EPOCH FROM now() - bt.timestamp)::int as age").
		From("sent_information_requests sr").
		Join("logs l on l.id = sr.log_id").
		Join("block_timestamps bt on bt.chain_id = l.chain_id AND bt.block_number = l.block_number").
		Join("information_requests r on sr.bridge_id = r.bridge_id AND r.message_id = sr.message_id").
		Join("signed_information_requests s on s.bridge_id = r.bridge_id AND r.message_id = s.message_id").
		Where(sq.GtOrEq{
			"l.block_number": params.HomeStartBlockNumber,
		}).
		Where(sq.Eq{
			"l.chain_id":   params.HomeChainID,
			"sr.bridge_id": params.Bridge,
		}).
		Having(sq.Gt{
			"count(DISTINCT s.data)": 1,
		}).
		GroupBy("l.id", "r.id", "bt.timestamp").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}

	res := make([]DifferentInformationSignature, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type UnknownInformationSignature struct {
	ChainID         string         `db:"chain_id"`
	BlockNumber     uint64         `db:"block_number"`
	Age             time.Duration  `db:"age"`
	TransactionHash common.Hash    `db:"transaction_hash"`
	Signer          common.Address `db:"signer"`
	MessageID       common.Hash    `db:"message_id"`
}

func (c *UnknownInformationSignature) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"signer":       c.Signer.String(),
			"message_id":   c.MessageID.String(),
		},
		Value: float64(c.Age),
	}
}

func (p *DBAlertsProvider) FindUnknownInformationSignatures(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	minProcessedTS, err := p.findMinProcessedTime(ctx, params)
	if err != nil {
		return nil, err
	}
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "sr.signer", "sr.message_id", "EXTRACT(EPOCH FROM now() - bt.timestamp)::int as age").
		From("signed_information_requests sr").
		Join("logs l ON l.id = sr.log_id").
		Join("block_timestamps bt on bt.chain_id = l.chain_id AND bt.block_number = l.block_number").
		LeftJoin("information_requests r ON sr.bridge_id = r.bridge_id AND r.message_id = sr.message_id").
		Where(sq.Eq{"r.id": nil, "sr.bridge_id": params.Bridge, "l.chain_id": params.HomeChainID}).
		Where(sq.GtOrEq{"l.block_number": params.HomeStartBlockNumber}).
		Where(sq.LtOrEq{"bt.timestamp": minProcessedTS}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := make([]UnknownInformationSignature, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type UnknownInformationExecution struct {
	ChainID         string        `db:"chain_id"`
	BlockNumber     uint64        `db:"block_number"`
	Age             time.Duration `db:"age"`
	TransactionHash common.Hash   `db:"transaction_hash"`
	MessageID       common.Hash   `db:"message_id"`
}

func (c *UnknownInformationExecution) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"message_id":   c.MessageID.String(),
		},
		Value: float64(c.Age),
	}
}

func (p *DBAlertsProvider) FindUnknownInformationExecutions(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	minProcessedTS, err := p.findMinProcessedTime(ctx, params)
	if err != nil {
		return nil, err
	}
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "er.message_id", "EXTRACT(EPOCH FROM now() - bt.timestamp)::int as age").
		From("executed_information_requests er").
		Join("logs l ON l.id = er.log_id").
		Join("block_timestamps bt on bt.chain_id = l.chain_id AND bt.block_number = l.block_number").
		LeftJoin("information_requests r ON er.bridge_id = r.bridge_id AND er.message_id = r.message_id").
		Where(sq.Eq{"r.id": nil, "er.bridge_id": params.Bridge, "l.chain_id": params.HomeChainID}).
		Where(sq.GtOrEq{"l.block_number": params.HomeStartBlockNumber}).
		Where(sq.LtOrEq{"bt.timestamp": minProcessedTS}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := make([]UnknownInformationExecution, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

func (p *DBAlertsProvider) FindUnknownErcToNativeConfirmations(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	minProcessedTS, err := p.findMinProcessedTime(ctx, params)
	if err != nil {
		return nil, err
	}
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "sm.signer", "sm.msg_hash", "EXTRACT(EPOCH FROM now() - bt.timestamp)::int as age").
		From("signed_messages sm").
		Join("logs l ON l.id = sm.log_id").
		Join("block_timestamps bt on bt.chain_id = l.chain_id AND bt.block_number = l.block_number").
		LeftJoin("erc_to_native_messages m ON sm.bridge_id = m.bridge_id AND m.msg_hash = sm.msg_hash").
		Where(sq.Eq{"m.id": nil, "sm.bridge_id": params.Bridge, "l.chain_id": params.HomeChainID}).
		Where(sq.GtOrEq{"l.block_number": params.HomeStartBlockNumber}).
		Where(sq.LtOrEq{"bt.timestamp": minProcessedTS}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := make([]UnknownConfirmation, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type UnknownErcToNativeExecution struct {
	ChainID         string        `db:"chain_id"`
	BlockNumber     uint64        `db:"block_number"`
	Age             time.Duration `db:"age"`
	TransactionHash common.Hash   `db:"transaction_hash"`
	MsgHash         common.Hash   `db:"msg_hash"`
}

func (c *UnknownErcToNativeExecution) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"msg_hash":     c.MsgHash.String(),
		},
		Value: float64(c.Age),
	}
}

func (p *DBAlertsProvider) FindUnknownErcToNativeExecutions(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	minProcessedTS, err := p.findMinProcessedTime(ctx, params)
	if err != nil {
		return nil, err
	}
	q, args, err := sq.Select("l.chain_id", "l.block_number", "l.transaction_hash", "em.message_id as msg_hash", "EXTRACT(EPOCH FROM now() - bt.timestamp)::int as age").
		From("executed_messages em").
		Join("logs l ON l.id = em.log_id").
		Join("block_timestamps bt on bt.chain_id = l.chain_id AND bt.block_number = l.block_number").
		LeftJoin("erc_to_native_messages m ON em.bridge_id = m.bridge_id AND em.message_id = m.msg_hash").
		Where(sq.Eq{"m.id": nil, "em.bridge_id": params.Bridge}).
		Where(sq.Or{
			sq.And{
				sq.Eq{"l.chain_id": params.HomeChainID},
				sq.GtOrEq{"l.block_number": params.HomeStartBlockNumber},
			},
			sq.And{
				sq.Eq{"l.chain_id": params.ForeignChainID},
				sq.GtOrEq{"l.block_number": params.ForeignStartBlockNumber},
			},
		}).
		Where(sq.LtOrEq{"bt.timestamp": minProcessedTS}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't build query: %w", err)
	}
	res := make([]UnknownErcToNativeExecution, 0, 5)
	err = p.db.SelectContext(ctx, &res, q, args...)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}

type StuckErcToNativeMessage struct {
	ChainID         string         `db:"chain_id"`
	BlockNumber     uint64         `db:"block_number"`
	Age             time.Duration  `db:"age"`
	TransactionHash common.Hash    `db:"transaction_hash"`
	MsgHash         common.Hash    `db:"msg_hash"`
	Count           uint64         `db:"count"`
	Receiver        common.Address `db:"receiver"`
	Value           string         `db:"value"`
}

func (c *StuckErcToNativeMessage) AlertValues() AlertValues {
	return AlertValues{
		Labels: map[string]string{
			"chain_id":     c.ChainID,
			"block_number": strconv.FormatUint(c.BlockNumber, 10),
			"tx_hash":      c.TransactionHash.String(),
			"msg_hash":     c.MsgHash.String(),
			"count":        strconv.FormatUint(c.Count, 10),
			"receiver":     c.Receiver.String(),
			"value":        c.Value,
		},
		Value: float64(c.Age),
	}
}

func (p *DBAlertsProvider) FindStuckErcToNativeMessages(ctx context.Context, params *AlertJobParams) ([]AlertValues, error) {
	query := `
		SELECT l.chain_id,
		       l.block_number,
		       l.transaction_hash,
		       sm.msg_hash,
		       count(s.log_id) as count,
		       EXTRACT(EPOCH FROM now() - ts.timestamp)::int as age,
		       m.receiver,
		       m.value / 1e18 as value
		FROM sent_messages sm
		         JOIN logs l on l.id = sm.log_id
		         JOIN block_timestamps ts on ts.chain_id = l.chain_id AND ts.block_number = l.block_number
		         JOIN erc_to_native_messages m on sm.bridge_id = m.bridge_id AND m.msg_hash = sm.msg_hash
		         LEFT JOIN signed_messages s on s.bridge_id = m.bridge_id AND m.msg_hash = s.msg_hash
		         LEFT JOIN collected_messages cm on m.bridge_id = cm.bridge_id AND cm.msg_hash = m.msg_hash
		WHERE m.direction::direction_enum = 'home_to_foreign'
		  AND cm.log_id IS NULL
		  AND sm.bridge_id = $1
		  AND l.block_number >= $2
		GROUP BY sm.log_id, l.id, ts.timestamp, m.id
		UNION
		SELECT l.chain_id,
		       l.block_number,
		       l.transaction_hash,
		       sm.msg_hash,
		       count(s.log_id) as count,
		       EXTRACT(EPOCH FROM now() - ts.timestamp)::int as age,
		       m.receiver,
		       m.value / 1e18 as value
		FROM sent_messages sm
		         JOIN logs l on l.id = sm.log_id
		         JOIN block_timestamps ts on ts.chain_id = l.chain_id AND ts.block_number = l.block_number
		         JOIN erc_to_native_messages m on sm.bridge_id = m.bridge_id AND m.msg_hash = sm.msg_hash
		         LEFT JOIN signed_messages s on s.bridge_id = m.bridge_id AND m.msg_hash = s.msg_hash
		         LEFT JOIN executed_messages em on m.bridge_id = em.bridge_id AND em.message_id = m.msg_hash
		WHERE m.direction::direction_enum = 'foreign_to_home'
		  AND em.log_id IS NULL
		  AND sm.bridge_id = $1
		  AND l.block_number >= $3
		  AND m.value > 0
		GROUP BY sm.log_id, l.id, ts.timestamp, m.id`
	res := make([]StuckErcToNativeMessage, 0, 5)
	err := p.db.SelectContext(ctx, &res, query, params.Bridge, params.HomeStartBlockNumber, params.ForeignStartBlockNumber)
	if err != nil {
		return nil, fmt.Errorf("can't select alerts: %w", err)
	}
	alerts := make([]AlertValues, len(res))
	for i := range res {
		alerts[i] = res[i].AlertValues()
	}
	return alerts, nil
}
