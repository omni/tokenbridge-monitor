package ethclient

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type Client struct {
	ChainID string
	url     string
	timeout time.Duration
	client  *ethclient.Client
}

func NewClient(url string, timeout time.Duration, chainID string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rawClient, err := rpc.DialContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("can't dial JSON rpc url: %w", err)
	}
	client := &Client{
		ChainID: chainID,
		url:     url,
		timeout: timeout,
		client:  ethclient.NewClient(rawClient),
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
	return client, nil
}

func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	defer ObserveDuration(c.ChainID, c.url, "eth_blockNumber")()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	n, err := c.client.BlockNumber(ctx)
	ObserveError(c.ChainID, c.url, "eth_getBlockByNumber", err)
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
