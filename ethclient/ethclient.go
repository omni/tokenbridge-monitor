package ethclient

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type Client struct {
	ChainID   string
	url       string
	timeout   time.Duration
	rawClient *rpc.Client
	client    *ethclient.Client
	signer    types.Signer
}

func NewClient(url string, timeout time.Duration, chainID string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rawClient, err := rpc.DialContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("can't dial JSON rpc url: %w", err)
	}
	client := &Client{
		ChainID:   chainID,
		url:       url,
		timeout:   timeout,
		rawClient: rawClient,
		client:    ethclient.NewClient(rawClient),
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), timeout)
	defer cancel2()
	rpcChainID, err := client.client.ChainID(ctx2)
	if err != nil {
		return nil, fmt.Errorf("can't get chainID: %w", err)
	}
	if rpcChainID.String() != chainID {
		return nil, fmt.Errorf("rpc url retunrned different chainID, expected %s, got %s", chainID, rpcChainID)
	}
	client.signer = types.NewLondonSigner(rpcChainID)
	return client, nil
}

func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	defer ObserveDuration(c.ChainID, c.url, "eth_blockNumber")()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	n, err := c.client.BlockNumber(ctx)
	ObserveError(c.ChainID, c.url, "eth_blockNumber", err)
	return n, err
}

func (c *Client) HeaderByNumber(ctx context.Context, n uint) (*types.Header, error) {
	defer ObserveDuration(c.ChainID, c.url, "eth_getBlockByNumber")()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	header, err := c.client.HeaderByNumber(ctx, big.NewInt(int64(n)))
	ObserveError(c.ChainID, c.url, "eth_getBlockByNumber", err)
	return header, err
}

func (c *Client) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	defer ObserveDuration(c.ChainID, c.url, "eth_getLogs")()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	logs, err := c.client.FilterLogs(ctx, q)
	ObserveError(c.ChainID, c.url, "eth_getLogs", err)
	return logs, err
}

// FilterLogsSafe is the same as FilterLogs, but makes an additional eth_blockNumber
// request to ensure that the node behind RPC is synced to the needed point.
func (c *Client) FilterLogsSafe(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	defer ObserveDuration(c.ChainID, c.url, "eth_getLogsSafe")()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var err error
	defer func() {
		ObserveError(c.ChainID, c.url, "eth_getLogsSafe", err)
	}()

	var arg interface{}
	arg, err = toFilterArg(q)
	if err != nil {
		return nil, fmt.Errorf("can't encode filter argument: %w", err)
	}
	var logs []types.Log
	var blockNumber hexutil.Uint64
	batches := []rpc.BatchElem{
		{
			Method: "eth_getLogs",
			Args:   []interface{}{arg},
			Result: &logs,
		},
		{
			Method: "eth_blockNumber",
			Result: &blockNumber,
		},
	}
	err = c.rawClient.BatchCallContext(ctx, batches)
	if err != nil {
		return nil, fmt.Errorf("can't make batch request: %w", err)
	}
	if err = batches[0].Error; err != nil {
		return nil, fmt.Errorf("can't request logs: %w", err)
	}
	if err = batches[1].Error; err != nil {
		return nil, fmt.Errorf("can't request block number: %w", err)
	}
	if uint64(blockNumber) < q.ToBlock.Uint64() {
		return nil, fmt.Errorf("node is not synced, current block %d is older than toBlock %d in the query", blockNumber, q.ToBlock.Uint64())
	}
	return logs, nil
}

func (c *Client) TransactionByHash(ctx context.Context, txHash common.Hash) (*types.Transaction, error) {
	defer ObserveDuration(c.ChainID, c.url, "eth_getTransactionByHash")()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	tx, _, err := c.client.TransactionByHash(ctx, txHash)
	ObserveError(c.ChainID, c.url, "eth_getTransactionByHash", err)
	return tx, err
}

func (c *Client) TransactionReceiptByHash(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	defer ObserveDuration(c.ChainID, c.url, "eth_getTransactionReceipt")()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	receipt, err := c.client.TransactionReceipt(ctx, txHash)
	ObserveError(c.ChainID, c.url, "eth_getTransactionReceipt", err)
	return receipt, err
}

func (c *Client) CallContract(ctx context.Context, msg ethereum.CallMsg) ([]byte, error) {
	defer ObserveDuration(c.ChainID, c.url, "eth_call")()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	res, err := c.client.CallContract(ctx, msg, nil)
	ObserveError(c.ChainID, c.url, "eth_call", err)
	return res, err
}

func (c *Client) TransactionSender(tx *types.Transaction) (common.Address, error) {
	return c.signer.Sender(tx)
}

func toFilterArg(q ethereum.FilterQuery) (interface{}, error) {
	arg := map[string]interface{}{
		"address": q.Addresses,
		"topics":  q.Topics,
	}
	if q.BlockHash != nil {
		return nil, fmt.Errorf("logs query from BlockHash is not supported")
	} else {
		if q.FromBlock == nil {
			arg["fromBlock"] = "0x0"
		} else {
			arg["fromBlock"] = hexutil.EncodeBig(q.FromBlock)
		}
		if q.ToBlock == nil || q.ToBlock.Int64() <= 0 {
			return nil, fmt.Errorf("only positive toBlock is supported")
		}
		arg["toBlock"] = hexutil.EncodeBig(q.ToBlock)
	}
	return arg, nil
}
