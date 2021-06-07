CREATE INDEX IF NOT EXISTS message_request_block_number_index
    ON message_request (block_number);

CREATE INDEX IF NOT EXISTS message_request_tx_hash_index
    ON message_request (tx_hash);

CREATE INDEX IF NOT EXISTS message_confirmation_block_number_index
    ON message_confirmation (block_number);

CREATE INDEX IF NOT EXISTS message_confirmation_tx_hash_index
    ON message_confirmation (tx_hash);

CREATE INDEX IF NOT EXISTS message_execution_block_number_index
    ON message_execution (block_number);

CREATE INDEX IF NOT EXISTS message_execution_tx_hash_index
    ON message_execution (tx_hash);