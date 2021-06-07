package ethclient

import (
	"context"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"time"
)

type Client struct {
	timeout time.Duration
	*ethclient.Client
}

func NewClient(url string, timeout int64) (*Client, error) {
	rawClient, err := ethclient.Dial(url)
	if err != nil {
		return nil, err
	}
	return &Client{time.Millisecond * time.Duration(timeout), rawClient}, nil
}

func (c *Client) GetCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.timeout)
}

func (c *Client) ChainID() (*big.Int, error) {
	ctx, cancel := c.GetCtx()
	defer cancel()
	return c.Client.ChainID(ctx)
}

func (c *Client) BlockNumber() (uint64, error) {
	ctx, cancel := c.GetCtx()
	defer cancel()
	return c.Client.BlockNumber(ctx)
}

func (c *Client) HeaderByNumber(n uint64) (*types.Header, error) {
	ctx, cancel := c.GetCtx()
	defer cancel()
	return c.Client.HeaderByNumber(ctx, big.NewInt(int64(n)))
}

func (c *Client) FilterLogs(q ethereum.FilterQuery) ([]types.Log, error) {
	ctx, cancel := c.GetCtx()
	defer cancel()
	return c.Client.FilterLogs(ctx, q)
}
