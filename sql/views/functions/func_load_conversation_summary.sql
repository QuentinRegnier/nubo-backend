CREATE OR REPLACE FUNCTION views.func_load_conversation_summary(
    p_conversation_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
RETURNS TABLE (
    conversation_id BIGINT,
    user_id BIGINT,
    role SMALLINT,
    joined_at TIMESTAMPTZ,
    unread_count INT,
    last_message_id BIGINT,
    last_sender_id BIGINT,
    last_message_type SMALLINT,
    last_message_state SMALLINT,
    last_message_content TEXT,
    last_message_time TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT *
    FROM public.conversation_summary
    WHERE 
        (p_conversation_id IS NULL OR conversation_summary.conversation_id = p_conversation_id)
        AND (p_user_id IS NULL OR conversation_summary.user_id = p_user_id);
END;
$$;