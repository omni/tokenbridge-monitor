ALTER TABLE erc_to_native_messages
    ADD COLUMN sender ADDRESS DEFAULT '\x0000000000000000000000000000000000000000'::ADDRESS;
UPDATE erc_to_native_messages m
SET m.sender = m.receiver
FROM sent_messages sm,
     logs l
WHERE m.bridge_id = sm.bridge_id
  AND m.msg_hash = sm.msg_hash
  AND l.id = sm.log_id
  AND l.topic0 = '\xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef'::OPT_WORD;
SET m.sender = substring(l2.topic2 from 12 for 20)::ADDRESS
FROM sent_messages sm,
     logs l,
     logs l2
WHERE m.bridge_id = sm.bridge_id
  AND m.msg_hash = sm.msg_hash
  AND l.id = sm.log_id
  AND l.topic0 = '\x127650bcfb0ba017401abe4931453a405140a8fd36fece67bae2db174d3fdd63'::OPT_WORD
  AND l2.transaction_hash = l.transaction_hash
  AND l2.topic0 = '\xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef'::OPT_WORD;

ALTER TABLE erc_to_native_messages
    ALTER COLUMN sender DROP DEFAULT;