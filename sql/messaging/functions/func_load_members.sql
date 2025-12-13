CREATE OR REPLACE FUNCTION messaging.func_load_members(
    p_conversation_id BIGINT
)
-- 1. Définition de la structure de sortie
RETURNS TABLE (
    user_id BIGINT,
    username TEXT,
    grade SMALLINT,
    banned BOOLEAN,
    desactivated BOOLEAN,
    profile_picture_storage_path TEXT,
    role SMALLINT,
    joined_at TIMESTAMPTZ,
    unread_count INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- 2. Utilisation de RETURN QUERY pour renvoyer l'ensemble des résultats
    RETURN QUERY
    SELECT
        u.id AS user_id,
        u.username,
        u.grade,
        u.banned,
        u.desactivated,
        med.storage_path AS profile_picture_storage_path,
        m.role,
        m.joined_at,
        m.unread_count
    FROM
        -- 3. Table de base : les membres de la conversation
        messaging.members AS m
    INNER JOIN
        -- 4. Jointure avec auth.users pour les infos utilisateur
        auth.users AS u ON m.user_id = u.id
    LEFT JOIN
        -- 5. LEFT JOIN avec content.media pour la photo (optionnelle)
        content.media AS med ON u.profile_picture_id = med.id
    WHERE
        -- 6. Filtre sur la conversation demandée
        m.conversation_id = p_conversation_id;
END;
$$;