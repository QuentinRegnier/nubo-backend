CREATE OR REPLACE PROCEDURE proc_update_conversation(
    p_conversation_id BIGINT,
    p_title TEXT DEFAULT NULL,
    p_last_message_id BIGINT DEFAULT NULL,
    p_state SMALLINT DEFAULT NULL,
    p_laws SMALLINT[] DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE conversations
    SET
        title = COALESCE(p_title, title),
        last_message_id = COALESCE(p_last_message_id, last_message_id),
        state = COALESCE(p_state, state),
        laws = COALESCE(p_laws, laws)
    WHERE id = p_conversation_id;
END;
$$;