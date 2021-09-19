package repository

import (
	"amb-monitor/db"
	"amb-monitor/entity"
	"amb-monitor/repository/postgres"
)

type Repo struct {
	LogsCursors      entity.LogsCursorsRepo
	Logs             entity.LogsRepo
	BlockTimestamps  entity.BlockTimestampsRepo
	Messages         entity.MessagesRepo
	SentMessages     entity.SentMessagesRepo
	SignedMessages   entity.SignedMessagesRepo
	ExecutedMessages entity.ExecutedMessagesRepo
}

func NewRepo(db *db.DB) *Repo {
	return &Repo{
		LogsCursors:      postgres.NewLogsCursorRepo("logs_cursors", db),
		Logs:             postgres.NewLogsRepo("logs", db),
		BlockTimestamps:  postgres.NewBlockTimestampsRepo("block_timestamps", db),
		Messages:         postgres.NewMessagesRepo("messages", db),
		SentMessages:     postgres.NewSentMessagesRepo("sent_messages", db),
		SignedMessages:   postgres.NewSignedMessagesRepo("signed_messages", db),
		ExecutedMessages: postgres.NewExecutedMessagesRepo("executed_messages", db),
	}
}
