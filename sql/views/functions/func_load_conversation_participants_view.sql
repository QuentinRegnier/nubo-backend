CREATE OR REPLACE FUNCTION views.func_load_conversation_participants_view(
    p_conversation_id UUID DEFAULT NULL,
    p_user_id UUID DEFAULT NULL
)
RETURNS TABLE (
    conversation_id UUID,
    user_id UUID,
    username TEXT,
    first_name TEXT,
    last_name TEXT,
    role SMALLINT,
    joined_at TIMESTAMPTZ,
    unread_count INT,
    conversation_type SMALLINT,
    conversation_title TEXT,
    conversation_state SMALLINT,
    conversation_created_at TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT *
    FROM public.conversation_participants_view
    WHERE 
        (p_conversation_id IS NULL OR conversation_participants_view.conversation_id = p_conversation_id)
        AND (p_user_id IS NULL OR conversation_participants_view.user_id = p_user_id);
END;
$$;