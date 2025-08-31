CREATE OR REPLACE FUNCTION func_create_user_conversation(
    p_conversation_id UUID,
    p_user_id UUID,
    p_role SMALLINT DEFAULT 0
) RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_member_id UUID;
BEGIN
    INSERT INTO messaging.conversation_members(conversation_id, user_id, role)
    VALUES (p_conversation_id, p_user_id, p_role)
    ON CONFLICT (conversation_id, user_id) DO UPDATE
        SET role = EXCLUDED.role
    RETURNING id INTO v_member_id;

    RETURN v_member_id;
END;
$$;