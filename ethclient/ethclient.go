package ethclient

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
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

	rawClient, err := ethclient.DialContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("can't dial JSON rpc url: %w", err)
	}
	client := &Client{
		ChainID: chainID,
		url:     url,
		timeout: timeout,
		client:  rawClient,
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
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.BlockNumber(ctx)
}

func (c *Client) HeaderByNumber(ctx context.Context, n uint) (*types.Header, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.HeaderByNumber(ctx, big.NewInt(int64(n)))
}

func (c *Client) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.FilterLogs(ctx, q)
}
