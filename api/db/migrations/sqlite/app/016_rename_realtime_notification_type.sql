-- app/016_rename_realtime_notification_type.sql: Rename the realtime notification channel away from legacy transport naming.

UPDATE notifications
SET notification_type = 'realtime'
WHERE notification_type = 'sse';

UPDATE dictionary_values
SET label = 'Realtime',
    description = 'Realtime WebSocket notification'
WHERE value_code = 'realtime'
  AND dictionary_type_id IN (
      SELECT id FROM dictionary_types WHERE type_key = 'notification_type'
  );

UPDATE dictionary_values
SET value_code = 'realtime',
    label = 'Realtime',
    description = 'Realtime WebSocket notification'
WHERE value_code = 'sse'
  AND dictionary_type_id IN (
      SELECT id FROM dictionary_types WHERE type_key = 'notification_type'
  )
  AND NOT EXISTS (
      SELECT 1
      FROM dictionary_values existing
      WHERE existing.dictionary_type_id = dictionary_values.dictionary_type_id
        AND existing.value_code = 'realtime'
  );

DELETE FROM dictionary_values
WHERE value_code = 'sse'
  AND dictionary_type_id IN (
      SELECT id FROM dictionary_types WHERE type_key = 'notification_type'
  );
