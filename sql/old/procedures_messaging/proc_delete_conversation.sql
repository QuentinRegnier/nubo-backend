CREATE OR REPLACE PROCEDURE proc_delete_conversation(
    p_conversation_id BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE messaging.conversations
    SET state = 1
    WHERE id = p_conversation_id;
END;
$$;