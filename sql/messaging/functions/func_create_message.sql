CREATE OR REPLACE FUNCTION messaging.func_create_message(
    p_conversation_id BIGINT,
    p_sender_id BIGINT,
    p_content TEXT,
    p_attachments JSONB,
    p_message_type SMALLINT DEFAULT 0,
    p_visibility BOOLEAN DEFAULT TRUE
) RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
    v_message_id BIGINT;
    v_user_id BIGINT;
BEGIN
    -- 1️⃣ Insertion du message
    INSERT INTO messaging.messages(conversation_id, sender_id, message_type, visibility, content, attachments)
    VALUES (p_conversation_id, p_sender_id, p_message_type, p_visibility, p_content, p_attachments)
    RETURNING id INTO v_message_id;

    -- 2️⃣ Met à jour la conversation via la procédure proc_update_conversation
    CALL proc_update_conversation(
        p_conversation_id := p_conversation_id,
        p_last_message_id := v_message_id
    );

    -- 3️⃣ Met à jour les unread_count pour tous les membres sauf l’expéditeur
    FOR v_user_id IN
        SELECT user_id
        FROM messaging.members
        WHERE conversation_id = p_conversation_id
          AND user_id <> p_sender_id
    LOOP
        CALL proc_update_member(
            p_conversation_id := p_conversation_id,
            p_user_id := v_user_id,
            p_unread_count := (
                SELECT unread_count + 1
                FROM messaging.members
                WHERE conversation_id = p_conversation_id
                  AND user_id = v_user_id
            )
        );
    END LOOP;

    -- 4️⃣ Retourne l’identifiant du message nouvellement créé
    RETURN v_message_id;
END;
$$;