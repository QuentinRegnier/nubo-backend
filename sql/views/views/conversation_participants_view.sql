CREATE OR REPLACE VIEW views.conversation_participants_view AS
SELECT
    cm.conversation_id,
    cm.user_id,
    u.username,
    u.first_name,
    u.last_name,
    cm.role,
    cm.joined_at,
    cm.unread_count,
    conv.type AS conversation_type,
    conv.title AS conversation_title,
    conv.state AS conversation_state, -- 'state' provient de 'conversations'
    conv.created_at AS conversation_created_at
FROM messaging.members cm
         JOIN messaging.conversations conv ON cm.conversation_id = conv.id
         JOIN auth.users u ON cm.user_id = u.id;