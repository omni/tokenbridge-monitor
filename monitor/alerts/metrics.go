package alerts

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	AlertUnknownMessageConfirmation = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "alert",
		Subsystem: "monitor",
		Name:      "unknown_message_confirmation",
		Help:      "Shows found unknown AMB message confirmation sent by some validator.",
	}, []string{"bridge_id", "chain_id", "block_number", "tx_hash", "signer", "msg_hash"})
	AlertUnknownMessageExecution = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "alert",
		Subsystem: "monitor",
		Name:      "unknown_message_execution",
		Help:      "Shows found unknown AMB message execution.",
	}, []string{"bridge_id", "chain_id", "block_number", "tx_hash", "message_id"})
	AlertStuckMessageConfirmation = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "alert",
		Subsystem: "monitor",
		Name:      "stuck_message_confirmation",
		Help:      "Shows AMB message for which signatures are still in the pending. Value is set to the seconds passed since message was sent.",
	}, []string{"bridge_id", "chain_id", "block_number", "tx_hash", "msg_hash", "count"})
	AlertFailedMessageExecution = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "alert",
		Subsystem: "monitor",
		Name:      "failed_message_execution",
		Help:      "Shows AMB message which execution has failed.",
	}, []string{"bridge_id", "chain_id", "block_number", "tx_hash", "sender", "executor"})
)
