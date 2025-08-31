CREATE OR REPLACE PROCEDURE proc_delete_user_conversation(
    p_conversation_id UUID,
    p_user_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM messaging.conversation_members
    WHERE conversation_id = p_conversation_id AND user_id = p_user_id;
END;
$$;
