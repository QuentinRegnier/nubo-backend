CREATE OR REPLACE FUNCTION func_create_message(
    p_conversation_id UUID,
    p_sender_id UUID,
    p_content TEXT,
    p_attachments JSONB,
    p_message_type SMALLINT DEFAULT 0,
    p_state SMALLINT DEFAULT 0
) RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_message_id UUID;
BEGIN
    -- Insérer message
    INSERT INTO messaging.messages(conversation_id, sender_id, message_type, state, content, attachments)
    VALUES (p_conversation_id, p_sender_id, p_message_type, p_state, p_content, p_attachments)
    RETURNING id INTO v_message_id;

    -- Mettre à jour la conversation
    UPDATE messaging.conversations_meta
    SET last_message_id = v_message_id
    WHERE id = p_conversation_id;

    -- Incrémenter unread_count
    UPDATE messaging.conversation_members
    SET unread_count = unread_count + 1
    WHERE conversation_id = p_conversation_id
      AND user_id <> p_sender_id;

    RETURN v_message_id;
END;
$$;