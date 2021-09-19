CREATE DOMAIN ADDRESS AS BYTEA NOT NULL CHECK (length(value) = 20);
CREATE DOMAIN OPT_WORD AS BYTEA CHECK (length(value) = 32);
CREATE DOMAIN WORD AS OPT_WORD NOT NULL;
CREATE DOMAIN BLOB AS BYTEA NOT NULL;
CREATE DOMAIN BYTE AS INT NOT NULL CHECK (value >= 0 AND value <= 255);
CREATE DOMAIN CHAIN_ID AS DECIMAL(78) NOT NULL CHECK (value > 0);
CREATE DOMAIN GAS AS INT NOT NULL;
CREATE DOMAIN FLAG AS BOOLEAN NOT NULL;
CREATE DOMAIN BLOCK_NUMBER AS INT NOT NULL;
CREATE DOMAIN TS AS TIMESTAMP WITHOUT TIME ZONE NOT NULL;
CREATE DOMAIN TS_NOW AS TS DEFAULT now();
CREATE DOMAIN TEXT_ID AS TEXT NOT NULL DEFAULT '';
CREATE TYPE DIRECTION_ENUM AS ENUM ('home_to_foreign', 'foreign_to_home', 'foreign_async_call');
CREATE DOMAIN DIRECTION AS DIRECTION_ENUM NOT NULL;

CREATE TABLE logs_cursors
(
    chain_id             CHAIN_ID,
    address              ADDRESS,
    last_fetched_block   BLOCK_NUMBER,
    last_processed_block BLOCK_NUMBER,
    updated_at           TS_NOW,
    created_at           TS_NOW,
    PRIMARY KEY (chain_id, address),
    CONSTRAINT processed_before_fetched CHECK ( last_processed_block <= last_fetched_block )
);

CREATE TABLE logs
(
    id               SERIAL PRIMARY KEY,
    chain_id         CHAIN_ID,
    address          ADDRESS,
    topic0           OPT_WORD,
    topic1           OPT_WORD,
    topic2           OPT_WORD,
    topic3           OPT_WORD,
    data             BLOB,
    block_number     BLOCK_NUMBER,
    log_index        BLOCK_NUMBER,
    transaction_hash WORD,
    updated_at       TS_NOW,
    created_at       TS_NOW
);

CREATE TABLE block_timestamps
(
    chain_id     CHAIN_ID,
    block_number BLOCK_NUMBER,
    timestamp    TS,
    updated_at   TS_NOW,
    created_at   TS_NOW,
    PRIMARY KEY (chain_id, block_number)
);

CREATE TABLE messages
(
    id         SERIAL PRIMARY KEY,
    bridge_id  TEXT_ID,
    msg_hash   WORD,
    message_id WORD,
    direction  DIRECTION,
    sender     ADDRESS,
    executor   ADDRESS,
    data       BLOB,
    data_type  BYTE,
    gas_limit  GAS,
    updated_at TS_NOW,
    created_at TS_NOW
);

CREATE TABLE sent_messages
(
    log_id     SERIAL REFERENCES logs PRIMARY KEY,
    bridge_id  TEXT_ID,
    msg_hash   WORD,
    updated_at TS_NOW,
    created_at TS_NOW
);

CREATE TABLE signed_messages
(
    log_id         SERIAL REFERENCES logs PRIMARY KEY,
    bridge_id      TEXT_ID,
    msg_hash       WORD,
    signer         ADDRESS,
    is_responsible FLAG,
    updated_at     TS_NOW,
    created_at     TS_NOW
);

CREATE TABLE executed_messages
(
    log_id     SERIAL REFERENCES logs PRIMARY KEY,
    bridge_id  TEXT_ID,
    message_id WORD,
    status     FLAG,
    updated_at TS_NOW,
    created_at TS_NOW
);

CREATE UNIQUE INDEX logs_chain_id_block_number_log_idx ON logs (chain_id, block_number, log_index);
CREATE UNIQUE INDEX messages_bridge_id_msg_hash_idx ON messages (bridge_id, msg_hash);
CREATE INDEX messages_bridge_id_message_idx ON messages (bridge_id, message_id);
CREATE UNIQUE INDEX sent_messages_bridge_id_msg_hash_idx ON sent_messages (bridge_id, msg_hash);
CREATE UNIQUE INDEX signed_messages_bridge_id_msg_hash_signer_idx ON signed_messages (bridge_id, msg_hash, signer);
CREATE INDEX executed_messages_bridge_id_msg_hash_idx ON executed_messages (bridge_id, message_id);
