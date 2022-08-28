package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"

	"github.com/omni/tokenbridge-monitor/config"
	"github.com/omni/tokenbridge-monitor/presenter/http/render"
)

type ctxKey int

const (
	bridgeCfgCtxKey ctxKey = iota
	chainCfgCtxKey
	fromBlockNumberCtxKey
	toBlockNumberCtxKey
	txHashCtxKey
	filterCtxKey
)

var ErrInvalidBlockNumber = errors.New("invalid block number parameter")

type FilterContext struct {
	ChainID   *string
	FromBlock *uint
	ToBlock   *uint
	TxHash    *common.Hash
}

func GetBridgeConfigMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bridgeID := chi.URLParam(r, "bridgeID")

			bridgeCfg, ok := cfg.Bridges[bridgeID]
			if !ok || bridgeCfg == nil {
				render.JSON(w, r, http.StatusNotFound, fmt.Sprintf("bridge with id %s not found", bridgeID))
				return
			}

			ctx := context.WithValue(r.Context(), bridgeCfgCtxKey, bridgeCfg)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func BridgeConfig(ctx context.Context) *config.BridgeConfig {
	if cfg, ok := ctx.Value(bridgeCfgCtxKey).(*config.BridgeConfig); ok {
		return cfg
	}
	return new(config.BridgeConfig)
}

func GetChainConfigMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			chainID := chi.URLParam(r, "chainID")

			if chainID == "" {
				chainID = r.URL.Query().Get("chainId")
				if chainID == "" {
					next.ServeHTTP(w, r)
					return
				}
			}

			chainCfg := cfg.GetChainConfig(chainID)
			if chainCfg == nil {
				render.JSON(w, r, http.StatusNotFound, fmt.Sprintf("chain with id %s not found", chainID))
				return
			}

			ctx := context.WithValue(r.Context(), chainCfgCtxKey, chainCfg)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetBlockNumberMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blockNumberStr := chi.URLParam(r, "blockNumber")
		query := r.URL.Query()

		if blockNumberStr == "" {
			blockNumberStr = query.Get("blockNumber")
		}

		var fromBlockStr, toBlockStr string
		if blockNumberStr == "" {
			fromBlockStr = query.Get("fromBlock")
			toBlockStr = query.Get("toBlock")
			if fromBlockStr == "" || toBlockStr == "" {
				next.ServeHTTP(w, r)
				return
			}
		} else {
			fromBlockStr = blockNumberStr
			toBlockStr = blockNumberStr
		}

		fromBlock, err := strconv.ParseUint(fromBlockStr, 10, 32)
		if err != nil {
			render.Error(w, r, fmt.Errorf("failed to parse blockNumber: %w", err))
			return
		}
		toBlock, err := strconv.ParseUint(toBlockStr, 10, 32)
		if err != nil {
			render.Error(w, r, fmt.Errorf("failed to parse blockNumber: %w", err))
			return
		}

		if fromBlock > toBlock {
			render.Error(w, r, fmt.Errorf("fromBlock should be less than toBlock: %w", ErrInvalidBlockNumber))
			return
		}
		if toBlock-fromBlock > 10000 {
			render.Error(w, r, fmt.Errorf("cannot request more than 10000 blocks in range: %w", ErrInvalidBlockNumber))
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, fromBlockNumberCtxKey, uint(fromBlock))
		ctx = context.WithValue(ctx, toBlockNumberCtxKey, uint(toBlock))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetTxHashMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		txHash := chi.URLParam(r, "txHash")

		if txHash == "" {
			txHash = r.URL.Query().Get("txHash")
			if txHash == "" {
				next.ServeHTTP(w, r)
				return
			}
		}

		ctx := context.WithValue(r.Context(), txHashCtxKey, common.HexToHash(txHash))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetFilterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		filter := &FilterContext{}

		if cfg, ok := ctx.Value(chainCfgCtxKey).(*config.ChainConfig); ok {
			filter.ChainID = &cfg.ChainID
		}
		if blockNumber, ok := ctx.Value(fromBlockNumberCtxKey).(uint); ok {
			filter.FromBlock = &blockNumber
		}
		if blockNumber, ok := ctx.Value(toBlockNumberCtxKey).(uint); ok {
			filter.ToBlock = &blockNumber
		}
		if txHash, ok := ctx.Value(txHashCtxKey).(common.Hash); ok {
			filter.TxHash = &txHash
		}

		ctx = context.WithValue(ctx, filterCtxKey, filter)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetFilterContext(ctx context.Context) *FilterContext {
	if cfg, ok := ctx.Value(filterCtxKey).(*FilterContext); ok {
		return cfg
	}
	return new(FilterContext)
}
