CREATE OR REPLACE PROCEDURE proc_delete_message(
    p_message_id UUID,
    p_user_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE messaging.messages
    SET state = 1
    WHERE id = p_message_id
      AND sender_id = p_user_id
      AND state = 0;
END;
$$;
