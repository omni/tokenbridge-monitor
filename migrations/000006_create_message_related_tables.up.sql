CREATE TABLE IF NOT EXISTS message_request
(
    id           serial
        constraint message_request_pk
            primary key,
    msg_id       int not null
        constraint message_request_message_id_fk references message (id)
            on update cascade on delete cascade,
    chain_id     decimal(78, 0),
    tx_hash      char(66),
    block_number int,
    log_index    int
);

CREATE TABLE IF NOT EXISTS message_confirmation
(
    id            serial
        constraint message_confirmation_pk
            primary key,
    msg_id        int
        constraint message_confirmation_message_id_fk references message (id)
            on update cascade on delete cascade,
    validator     char(42),
    chain_id      decimal(78, 0),
    tx_hash       char(66),
    block_number  int,
    log_index     int,
    tmp_bridge_id varchar(32),
    tmp_msg_hash  char(66)
);

CREATE TABLE IF NOT EXISTS message_execution
(
    id             serial
        constraint message_execution_pk
            primary key,
    msg_id         int
        constraint message_execution_message_id_fk references message (id)
            on update cascade on delete cascade,
    status         bool,
    chain_id       decimal(78, 0),
    tx_hash        char(66),
    block_number   int,
    log_index      int,
    tmp_bridge_id  varchar(32),
    tmp_message_id char(66)
);
