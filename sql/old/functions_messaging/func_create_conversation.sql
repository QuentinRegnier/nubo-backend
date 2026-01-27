CREATE OR REPLACE FUNCTION messaging.func_create_conversation(
    p_type SMALLINT,
    p_title TEXT DEFAULT NULL,
    p_laws SMALLINT[] DEFAULT '{}',
    p_members JSONB -- tableau JSON du type [{ "user_id": "...", "role": 2}, ...]
) RETURNS BIGINT AS $$
DECLARE
    v_conversation_id BIGINT;
    v_user_ids BIGINT[];
    v_roles SMALLINT[];
BEGIN
    -- 1️⃣ Création de la conversation
    INSERT INTO messaging.conversations (type, title, laws)
    VALUES (p_type, p_title, p_laws)
    RETURNING id INTO v_conversation_id;

    -- 2️⃣ Extraction des données membres depuis le JSON fourni
    SELECT
        array_agg((m->>'user_id')::BIGINT),
        array_agg(COALESCE((m->>'role')::SMALLINT, 0))
    INTO v_user_ids, v_roles
    FROM jsonb_array_elements(p_members) AS m;

    -- 3️⃣ Appel à la fonction d’ajout multiple de membres
    PERFORM messaging.func_create_members_bulk(
        v_conversation_id,
        v_user_ids,
        v_roles
    );

    -- 4️⃣ Retourne l’ID de la conversation nouvellement créée
    RETURN v_conversation_id;
END;
$$ LANGUAGE plpgsql;