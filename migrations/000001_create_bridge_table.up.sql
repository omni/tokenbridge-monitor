CREATE TABLE IF NOT EXISTS bridge
(
    id            serial
        constraint bridge_pk
            primary key,
    chain_id      decimal(78, 0),
    address       char(42),
    start_block   int,
    current_block int
);