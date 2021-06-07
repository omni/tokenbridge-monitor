CREATE UNIQUE INDEX IF NOT EXISTS message_bridge_id_msg_hash_message_id_unique_index
    ON message (bridge_id, message_id, msg_hash);

CREATE UNIQUE INDEX IF NOT EXISTS message_request_msg_id_unique_index
    ON message_request (msg_id);

CREATE UNIQUE INDEX IF NOT EXISTS message_confirmation_tx_hash_validator_unique_index
    ON message_confirmation (tx_hash, validator);

CREATE UNIQUE INDEX IF NOT EXISTS message_execution_msg_id_unique_index
    ON message_execution (msg_id);
