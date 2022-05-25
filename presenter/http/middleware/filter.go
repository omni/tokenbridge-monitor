package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/presenter/http/render"
)

type ctxKey int

const (
	bridgeCfgCtxKey ctxKey = iota
	chainCfgCtxKey
	blockNumberCtxKey
	txHashCtxKey
	filterCtxKey
)

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

			var chainCfg *config.ChainConfig
			for _, c := range cfg.Chains {
				if c.ChainID == chainID {
					chainCfg = c
					break
				}
			}
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
		blockNumber, err := strconv.ParseUint(chi.URLParam(r, "blockNumber"), 10, 32)
		if err != nil {
			render.Error(w, r, fmt.Errorf("failed to parse blockNumber: %w", err))
			return
		}

		ctx := context.WithValue(r.Context(), blockNumberCtxKey, uint(blockNumber))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetTxHashMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		txHash := chi.URLParam(r, "txHash")

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
		if blockNumber, ok := ctx.Value(blockNumberCtxKey).(uint); ok {
			filter.FromBlock = &blockNumber
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
