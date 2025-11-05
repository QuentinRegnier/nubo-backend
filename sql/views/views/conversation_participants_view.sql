CREATE OR REPLACE VIEW conversation_user_view AS
SELECT
    cm.user_id,
    ARRAY_AGG(cm.conversation_id ORDER BY last_msg.created_at DESC) AS conversation_ids
FROM messaging.conversation_members cm
JOIN messaging.conversations_meta conv ON conv.id = cm.conversation_id
LEFT JOIN LATERAL (
    SELECT m.id, m.created_at, m.sender_id
    FROM messaging.messages m
    WHERE m.conversation_id = cm.conversation_id
      AND m.sender_id != cm.user_id
    ORDER BY m.created_at DESC
    LIMIT 1
) last_msg ON true
GROUP BY cm.user_id;