CREATE OR REPLACE PROCEDURE proc_delete_member(
    p_conversation_id BIGINT,
    p_user_id BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM messaging.members
    WHERE conversation_id = p_conversation_id AND user_id = p_user_id;
END;
$$;