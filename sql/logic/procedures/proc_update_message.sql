CREATE OR REPLACE PROCEDURE proc_update_message(
    p_message_id UUID,
    p_user_id UUID,
    p_content TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE messaging.messages
    SET content = p_content,
        updated_at = now()
    WHERE id = p_message_id
      AND sender_id = p_user_id
      AND message_type = 0 -- uniquement texte
      AND state = 0;       -- actif
END;
$$;
