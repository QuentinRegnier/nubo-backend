CREATE OR REPLACE PROCEDURE proc_delete_messages(
    p_message_ids BIGINT[],
    p_user_id BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE messaging.messages
    SET visibility = FALSE
    WHERE id = ANY(p_message_ids)
      AND sender_id = p_user_id
      AND visibility = TRUE;
END;
$$;