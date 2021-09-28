package alerts

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	NewAlertUnknownMessageConfirmation = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "unknown_message_confirmation",
			Help:        "Shows found unknown AMB message confirmation sent by some validator.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "signer", "msg_hash"})
	}
	NewAlertUnknownMessageExecution = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "unknown_message_execution",
			Help:        "Shows found unknown AMB message execution.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "message_id"})
	}
	NewAlertStuckMessageConfirmation = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "stuck_message_confirmation",
			Help:        "Shows AMB message for which signatures are still in the pending. Value is set to the seconds passed since message was sent.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "msg_hash", "count"})
	}
	NewAlertFailedMessageExecution = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "failed_message_execution",
			Help:        "Shows AMB message which execution has failed.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "sender", "executor"})
	}
)
