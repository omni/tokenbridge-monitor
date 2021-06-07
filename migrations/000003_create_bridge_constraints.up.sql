CREATE UNIQUE INDEX IF NOT EXISTS bridge_chain_id_address_uindex
    ON bridge (chain_id, address);