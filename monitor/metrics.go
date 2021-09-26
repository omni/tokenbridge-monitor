package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	LatestHeadBlock = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "monitor",
		Subsystem: "contract",
		Name:      "latest_head_block",
		Help:      "Shows the latest fetched head block for the particular contract. Logs up to this block are waiting to be fetched.",
	}, []string{"bridge_id", "chain_id", "address"})
	LatestFetchedBlock = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "monitor",
		Subsystem: "contract",
		Name:      "latest_fetched_block",
		Help:      "Shows the latest fetched block for the particular contract. Logs up to this block are already fetched and saved to the DB.",
	}, []string{"bridge_id", "chain_id", "address"})
	LatestProcessedBlock = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "monitor",
		Subsystem: "contract",
		Name:      "latest_processed_block",
		Help:      "Shows the latest processed block for the particular contract. Logs up to this block are already parsed and processed to AMB DB records.",
	}, []string{"bridge_id", "chain_id", "address"})
	SyncedContract = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "monitor",
		Subsystem: "contract",
		Name:      "synced",
		Help:      "Shows 1 if the contract is considered as synced up to chain head.",
	}, []string{"bridge_id", "chain_id", "address"})
)
