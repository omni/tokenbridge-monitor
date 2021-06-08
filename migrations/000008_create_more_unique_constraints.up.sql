CREATE UNIQUE INDEX IF NOT EXISTS message_bridge_id_msg_hash_message_id_unique_index
    ON message (bridge_id, message_id, msg_hash);

CREATE UNIQUE INDEX IF NOT EXISTS message_request_chain_id_tx_hash_log_index_unique_index
    ON message_request (chain_id, tx_hash, log_index);

CREATE UNIQUE INDEX IF NOT EXISTS message_confirmation_chain_id_tx_hash_log_index_unique_index
    ON message_confirmation (chain_id, tx_hash, log_index);

CREATE UNIQUE INDEX IF NOT EXISTS message_execution_chain_id_tx_hash_unique_index
    ON message_execution (chain_id, tx_hash, log_index);
