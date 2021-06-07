CREATE TABLE IF NOT EXISTS message
(
    id         serial
        constraint message_pk
            primary key,
    msg_hash   char(66),
    bridge_id  varchar(32)
        constraint message_bridge_pair_id_fk references bridge_pair (id)
            on update cascade on delete cascade,
    direction  bool,
    message_id char(66),
    sender     char(42),
    executor   char(42),
    gas_limit  int,
    data_type  int8,
    data       text
);
