CREATE OR REPLACE FUNCTION func_load_messages(
    p_conversation_id UUID
) RETURNS SETOF messaging.messages
LANGUAGE sql
AS $$
    SELECT *
    FROM messaging.messages
    WHERE conversation_id = p_conversation_id
    ORDER BY created_at ASC;
$$;