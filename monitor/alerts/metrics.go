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
			Help:        "Shows AMB message for which signatures are still in the pending state.",
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
	NewAlertUnknownInformationSignature = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "unknown_information_signature",
			Help:        "Shows unknown AMB information request signatures sent by some validator.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "signer", "message_id"})
	}
	NewAlertUnknownInformationExecution = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "unknown_information_execution",
			Help:        "Shows unknown AMB information request executions.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "message_id"})
	}
	NewAlertStuckInformationRequest = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "stuck_information_request",
			Help:        "Shows AMB information requests for which signatures are still in the pending state.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "message_id", "count"})
	}
	NewAlertFailedInformationRequest = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "failed_information_request",
			Help:        "Shows AMB information requests which execution or callback has failed.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "sender", "executor", "status", "callback_status"})
	}
	NewAlertDifferentInformationSignatures = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "different_information_signatures",
			Help:        "Shows AMB information request signatures for which different validators submitted different results.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "message_id", "count"})
	}
	NewAlertUnknownErcToNativeMessageConfirmation = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "unknown_erc_to_native_message_confirmation",
			Help:        "Shows found unknown ERC_TO_NATIVE message confirmation sent by some validator.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "signer", "msg_hash"})
	}
	NewAlertUnknownErcToNativeMessageExecution = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "unknown_erc_to_native_message_execution",
			Help:        "Shows found unknown ERC_TO_NATIVE message execution.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "msg_hash"})
	}
	NewAlertStuckErcToNativeMessageConfirmation = func(bridge string) *prometheus.GaugeVec {
		return promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "alert",
			Subsystem:   "monitor",
			Name:        "stuck_erc_to_native_message_confirmation",
			Help:        "Shows ERC_TO_NATIVE message for which signatures are still in the pending state.",
			ConstLabels: prometheus.Labels{"bridge_id": bridge},
		}, []string{"chain_id", "block_number", "tx_hash", "msg_hash", "count", "receiver", "value"})
	}
)
