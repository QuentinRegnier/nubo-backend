CREATE OR REPLACE VIEW views.conversation_user_view AS
SELECT
    cm.user_id,
    ARRAY_AGG(cm.conversation_id ORDER BY last_msg.created_at DESC NULLS LAST) AS conversation_ids
FROM messaging.members cm
         LEFT JOIN LATERAL (
    SELECT m.created_at
    FROM messaging.messages m
    WHERE m.conversation_id = cm.conversation_id
      AND m.sender_id != cm.user_id
ORDER BY m.created_at DESC
    LIMIT 1
    ) last_msg ON true
GROUP BY cm.user_id;