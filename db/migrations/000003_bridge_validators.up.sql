CREATE TABLE bridge_validators
(
    log_id         SERIAL REFERENCES logs PRIMARY KEY,
    bridge_id      TEXT_ID,
    chain_id       CHAIN_ID,
    address        ADDRESS,
    removed_log_id INT REFERENCES logs NULL,
    updated_at     TS_NOW,
    created_at     TS_NOW
);
CREATE INDEX signed_messages_bridge_id_signer_idx ON signed_messages (bridge_id, signer);