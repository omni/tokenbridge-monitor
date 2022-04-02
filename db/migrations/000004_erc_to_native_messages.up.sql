CREATE DOMAIN UINT AS DECIMAL(80, 0) NOT NULL DEFAULT 0;
CREATE TABLE erc_to_native_messages
(
    id         SERIAL PRIMARY KEY,
    bridge_id  TEXT_ID,
    msg_hash   WORD,
    direction  DIRECTION,
    receiver   ADDRESS,
    value      UINT,
    updated_at TS_NOW,
    created_at TS_NOW
);
CREATE UNIQUE INDEX erc_to_native_messages_bridge_id_msg_hash_idx ON erc_to_native_messages (bridge_id, msg_hash);
DROP INDEX sent_messages_bridge_id_msg_hash_idx;
CREATE INDEX sent_messages_bridge_id_msg_hash_idx ON sent_messages (bridge_id, msg_hash);