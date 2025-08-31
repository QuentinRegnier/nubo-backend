-- Vue : conversation_summary
-- Résumé d’une conversation par membre :
-- - Dernier message
-- - Nombre de messages non lus
-- - Rôle et date d’adhésion

CREATE OR REPLACE VIEW conversation_summary AS
SELECT
    cm.conversation_id,
    cm.user_id,
    cm.role,
    cm.joined_at,
    cm.unread_count,
    last_msg.id AS last_message_id,
    last_msg.sender_id AS last_sender_id,
    last_msg.message_type AS last_message_type,
    last_msg.state AS last_message_state,
    last_msg.content AS last_message_content,
    last_msg.created_at AS last_message_time
FROM messaging.conversation_members cm
LEFT JOIN LATERAL (
    SELECT m.*
    FROM messaging.messages m
    WHERE m.conversation_id = cm.conversation_id
    ORDER BY m.created_at DESC
    LIMIT 1
) last_msg ON true;