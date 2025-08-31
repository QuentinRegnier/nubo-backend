CREATE OR REPLACE PROCEDURE proc_update_user_role_conversation(
    p_conversation_id UUID,
    p_user_id UUID,
    p_role SMALLINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE conversation_members
    SET role = p_role
    WHERE conversation_id = p_conversation_id AND user_id = p_user_id;
END;
$$;
