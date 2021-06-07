CREATE TABLE IF NOT EXISTS bridge_pair
(
    id             varchar(32)
        constraint bridge_pair_pk
            primary key,
    home_bridge    int
        constraint bridge_pair_bridge_id_fk
            references bridge (id)
            on update cascade on delete cascade,
    foreign_bridge int
        constraint bridge_pair_bridge_id_fk_2
            references bridge (id)
            on update cascade on delete cascade
);