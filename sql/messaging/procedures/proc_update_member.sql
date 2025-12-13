CREATE OR REPLACE PROCEDURE proc_update_member(
    p_conversation_id BIGINT,
    p_user_id BIGINT,
    p_role SMALLINT DEFAULT NULL,
    p_unread_count INT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE members
    SET
        role = COALESCE(p_role, role),
        unread_count = COALESCE(p_unread_count, unread_count)
    WHERE conversation_id = p_conversation_id
      AND user_id = p_user_id;
END;
$$;