-- message stats

-- total
SELECT count(*)
FROM messages
WHERE bridge_id = '?'
  AND direction = 'home_to_foreign';
SELECT count(*)
FROM messages
WHERE bridge_id = '?'
  AND direction = 'foreign_to_home';

-- foreign to home pending
SELECT m.*
FROM messages m
         LEFT JOIN executed_messages em ON m.id = em.message_id AND m.bridge_id = em.bridge_id
WHERE m.bridge_id = '?'
  AND m.direction = 'foreign_to_home'
  AND em.log_id IS NULL;

-- home to foreign oracle-driven pending
SELECT m.*
FROM messages m
         LEFT JOIN executed_messages em ON m.id = em.message_id AND m.bridge_id = em.bridge_id
WHERE m.bridge_id = '?'
  AND m.direction = 'home_to_foreign'
  AND m.data_type = 0
  AND em.log_id IS NULL;