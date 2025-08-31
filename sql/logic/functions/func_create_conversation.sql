CREATE OR REPLACE FUNCTION messaging.func_create_conversation(
    p_type SMALLINT,
    p_title TEXT,
    p_members JSONB -- tableau JSON du type [{ "user_id": "...", "role": 2}, ...]
) RETURNS UUID AS $$
DECLARE
    v_conversation_id UUID;
    v_member JSONB;
BEGIN
    -- 1️⃣ Conversation meta
    INSERT INTO messaging.conversations_meta (type, title)
    VALUES (p_type, p_title)
    RETURNING id INTO v_conversation_id;

    -- 2️⃣ Ajout des membres
    FOR v_member IN SELECT * FROM jsonb_array_elements(p_members)
    LOOP
        INSERT INTO messaging.conversation_members (conversation_id, user_id, role)
        VALUES (
            v_conversation_id,
            (v_member->>'user_id')::UUID,
            COALESCE((v_member->>'role')::SMALLINT, 0)
        );
    END LOOP;

    RETURN v_conversation_id;
END;
$$ LANGUAGE plpgsql;
