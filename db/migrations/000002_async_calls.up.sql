CREATE TABLE information_requests
(
    id               SERIAL PRIMARY KEY,
    bridge_id        TEXT_ID,
    message_id       WORD,
    direction        DIRECTION,
    request_selector WORD,
    sender           ADDRESS,
    executor         ADDRESS,
    data             BLOB,
    updated_at       TS_NOW,
    created_at       TS_NOW
);

CREATE TABLE sent_information_requests
(
    log_id     SERIAL REFERENCES logs PRIMARY KEY,
    bridge_id  TEXT_ID,
    message_id WORD,
    updated_at TS_NOW,
    created_at TS_NOW
);

CREATE TABLE signed_information_requests
(
    log_id     SERIAL REFERENCES logs PRIMARY KEY,
    bridge_id  TEXT_ID,
    message_id WORD,
    signer     ADDRESS,
    data       BLOB,
    updated_at TS_NOW,
    created_at TS_NOW
);

CREATE TABLE executed_information_requests
(
    log_id          SERIAL REFERENCES logs PRIMARY KEY,
    bridge_id       TEXT_ID,
    message_id      WORD,
    status          FLAG,
    callback_status FLAG,
    data            BLOB,
    updated_at      TS_NOW,
    created_at      TS_NOW
);

CREATE UNIQUE INDEX information_requests_bridge_id_message_id_idx ON information_requests (bridge_id, message_id);
CREATE UNIQUE INDEX sent_information_requests_bridge_id_message_id_idx ON sent_information_requests (bridge_id, message_id);
CREATE INDEX signed_information_requests_bridge_id_message_id_signer_idx ON signed_information_requests (bridge_id, message_id, signer);
CREATE UNIQUE INDEX executed_information_requests_bridge_id_message_id_idx ON executed_information_requests (bridge_id, message_id);

