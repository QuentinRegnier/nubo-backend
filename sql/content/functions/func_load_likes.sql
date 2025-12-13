CREATE OR REPLACE FUNCTION content.func_load_likes(
    p_target_type SMALLINT DEFAULT NULL,  -- 0=post, 1=message, 2=commentaire, NULL = tous
    p_target_id BIGINT DEFAULT NULL,        -- id de la cible (si ciblé)
    p_user_id BIGINT DEFAULT NULL,          -- id utilisateur (si on veut les likes d'un user)
    p_limit INT DEFAULT 100,              -- limite de résultats
    p_order_mode SMALLINT DEFAULT 0       -- 0=plus récents, 1=plus anciens
)
RETURNS TABLE(
    id BIGINT,
    target_type SMALLINT,
    target_id BIGINT,
    user_id BIGINT,
    created_at TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT
        l.id,
        l.target_type,
        l.target_id,
        l.user_id,
        l.created_at
    FROM content.likes l
    WHERE
        (p_target_type IS NULL OR l.target_type = p_target_type)
        AND (p_target_id IS NULL OR l.target_id = p_target_id)
        AND (p_user_id IS NULL OR l.user_id = p_user_id)
    ORDER BY
        CASE WHEN p_order_mode = 0 THEN l.created_at END DESC,
        CASE WHEN p_order_mode = 1 THEN l.created_at END ASC
    LIMIT p_limit;
END;
$$;