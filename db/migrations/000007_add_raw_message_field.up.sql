ALTER TABLE messages
    ADD COLUMN raw_message BYTEA;
ALTER TABLE erc_to_native_messages
    ADD COLUMN raw_message BYTEA;

UPDATE messages m
SET raw_message = l.transaction_hash || substr(l.data, 65, get_byte(l.data, 62) * 256 + get_byte(l.data, 63))
FROM sent_messages sm
         JOIN logs l
              ON l.id = sm.log_id
WHERE m.bridge_id = sm.bridge_id
  AND m.msg_hash = sm.msg_hash
  AND l.topic0 IN (
                   E'\\x9df71e9d2175354c68d0e882702c9eb5d63e036345c53639b8c63be4e8764741',
                   E'\\x733b62005ae93e850dd2b37234e1c7eb634d3b7de068bc0f7f32b7233191a48c'
    );

UPDATE messages m
SET raw_message = substr(l.data, 65, get_byte(l.data, 62) * 256 + get_byte(l.data, 63))
FROM sent_messages sm
         JOIN logs l
              ON l.id = sm.log_id
WHERE m.bridge_id = sm.bridge_id
  AND m.msg_hash = sm.msg_hash
  AND l.topic0 IN (
                   E'\\x482515ce3d9494a37ce83f18b72b363449458435fafdd7a53ddea7460fe01b58',
                   E'\\x520d2afde79cbd5db58755ac9480f81bc658e5c517fcae7365a3d832590b0183'
    );

UPDATE erc_to_native_messages m
SET raw_message = substr(l.data, 13) || l.transaction_hash || E'\\x4aa42145Aa6Ebf72e164C9bBC74fbD3788045016'
FROM sent_messages sm
         JOIN logs l
              ON l.id = sm.log_id
WHERE m.bridge_id = sm.bridge_id
  AND m.msg_hash = sm.msg_hash
  AND m.direction::direction_enum = 'home_to_foreign';

UPDATE erc_to_native_messages m
SET raw_message = substr(l.data, 13) || l.transaction_hash
FROM sent_messages sm
         JOIN logs l
              ON l.id = sm.log_id
WHERE m.bridge_id = sm.bridge_id
  AND m.msg_hash = sm.msg_hash
  AND m.direction::direction_enum = 'foreign_to_home'
  AND l.topic0 = E'\\x1d491a427d1f8cc0d447496f300fac39f7306122481d8e663451eb268274146b';

UPDATE erc_to_native_messages m
SET raw_message = substr(l.topic1, 13) || l.data || l.transaction_hash
FROM sent_messages sm
         JOIN logs l
              ON l.id = sm.log_id
WHERE m.bridge_id = sm.bridge_id
  AND m.msg_hash = sm.msg_hash
  AND m.direction::direction_enum = 'foreign_to_home'
  AND l.topic0 = E'\\xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef';

ALTER TABLE messages
    ALTER COLUMN raw_message TYPE BLOB;
ALTER TABLE erc_to_native_messages
    ALTER COLUMN raw_message TYPE BLOB;
