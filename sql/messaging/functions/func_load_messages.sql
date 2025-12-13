CREATE OR REPLACE FUNCTION messaging.func_load_messages(
    p_conversation_id BIGINT
) RETURNS SETOF messaging.messages
LANGUAGE sql
AS $$
    SELECT *
    FROM messaging.messages
    WHERE conversation_id = p_conversation_id
      AND visibility = TRUE  -- exclut les messages supprimés
    ORDER BY created_at ASC;
$$;