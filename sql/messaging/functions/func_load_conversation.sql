CREATE OR REPLACE FUNCTION messaging.func_load_conversation(
    p_user_id UUID
)
-- 1. Définition de la structure de sortie ("liste de structure")
RETURNS TABLE (
    conversation_id UUID,
    title TEXT,
    type SMALLINT,
    laws SMALLINT[],
    state SMALLINT,
    created_at TIMESTAMPTZ,
    last_message_id UUID,
    last_read_by_all_message_id UUID,
    joined_at TIMESTAMPTZ,
    role SMALLINT,
    unread_count INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- 2. Utilisation de RETURN QUERY pour renvoyer l'ensemble des résultats
    RETURN QUERY
    SELECT
        c.id AS conversation_id,
        c.title,
        c.type,
        c.laws,
        c.state,
        c.created_at,
        c.last_message_id,
        c.last_read_by_all_message_id,
        m.joined_at,
        m.role,
        m.unread_count
    FROM
        messaging.members AS m
    -- 3. Jointure pour combiner les tables
    INNER JOIN
        messaging.conversations AS c ON m.conversation_id = c.id
    WHERE
        -- 4. Filtre sur l'utilisateur demandé
        m.user_id = p_user_id
        -- 5. MODIFICATION : Exclut les conversations avec state = 1
        AND c.state <> 1;
END;
$$;