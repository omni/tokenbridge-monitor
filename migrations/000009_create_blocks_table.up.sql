CREATE TABLE IF NOT EXISTS block
(
    id           serial
        constraint block_pk
            primary key,
    chain_id     decimal(78, 0),
    block_number int,
    timestamp    timestamp
);

CREATE UNIQUE INDEX IF NOT EXISTS block_chain_id_block_number_unique_index
    ON block (chain_id, block_number);