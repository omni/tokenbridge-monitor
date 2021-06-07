ALTER TABLE bridge_pair
    ADD CONSTRAINT bridge_pair_neq_bridges
        CHECK ( home_bridge != foreign_bridge );
