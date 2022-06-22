package repository

import (
	"context"
	"fmt"

	"github.com/poanetwork/tokenbridge-monitor/config"
	"github.com/poanetwork/tokenbridge-monitor/db"
	"github.com/poanetwork/tokenbridge-monitor/entity"
	"github.com/poanetwork/tokenbridge-monitor/repository/postgres"
)

type Repo struct {
	LogsCursors                 entity.LogsCursorsRepo
	Logs                        entity.LogsRepo
	BlockTimestamps             entity.BlockTimestampsRepo
	Messages                    entity.MessagesRepo
	ErcToNativeMessages         entity.ErcToNativeMessagesRepo
	SentMessages                entity.SentMessagesRepo
	SignedMessages              entity.SignedMessagesRepo
	CollectedMessages           entity.CollectedMessagesRepo
	ExecutedMessages            entity.ExecutedMessagesRepo
	InformationRequests         entity.InformationRequestsRepo
	SentInformationRequests     entity.SentInformationRequestsRepo
	SignedInformationRequests   entity.SignedInformationRequestsRepo
	ExecutedInformationRequests entity.ExecutedInformationRequestsRepo
	BridgeValidators            entity.BridgeValidatorsRepo
}

func NewRepo(db *db.DB) *Repo {
	return &Repo{
		LogsCursors:                 postgres.NewLogsCursorRepo("logs_cursors", db),
		Logs:                        postgres.NewLogsRepo("logs", db),
		BlockTimestamps:             postgres.NewBlockTimestampsRepo("block_timestamps", db),
		Messages:                    postgres.NewMessagesRepo("messages", db),
		ErcToNativeMessages:         postgres.NewErcToNativeMessagesRepo("erc_to_native_messages", db),
		SentMessages:                postgres.NewSentMessagesRepo("sent_messages", db),
		SignedMessages:              postgres.NewSignedMessagesRepo("signed_messages", db),
		CollectedMessages:           postgres.NewCollectedMessagesRepo("collected_messages", db),
		ExecutedMessages:            postgres.NewExecutedMessagesRepo("executed_messages", db),
		InformationRequests:         postgres.NewInformationRequestsRepo("information_requests", db),
		SentInformationRequests:     postgres.NewSentInformationRequestsRepo("sent_information_requests", db),
		SignedInformationRequests:   postgres.NewSignedInformationRequestsRepo("signed_information_requests", db),
		ExecutedInformationRequests: postgres.NewExecutedInformationRequestsRepo("executed_information_requests", db),
		BridgeValidators:            postgres.NewBridgeValidatorsRepo("bridge_validators", db),
	}
}

func (r *Repo) FindPendingMessages(ctx context.Context, bridgeID string, bridgeMode config.BridgeMode) ([]entity.BridgeMessage, error) {
	if bridgeMode == config.BridgeModeErcToNative {
		msgs, err := r.ErcToNativeMessages.FindPendingMessages(ctx, bridgeID)
		if err != nil {
			return nil, fmt.Errorf("can't find pending erc-to-native messages: %w", err)
		}
		return entity.ToBridgeMessages(msgs), nil
	}
	msgs, err := r.Messages.FindPendingMessages(ctx, bridgeID)
	if err != nil {
		return nil, fmt.Errorf("can't find pending amb messages: %w", err)
	}
	return entity.ToBridgeMessages(msgs), nil
}
