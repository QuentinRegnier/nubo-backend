CREATE OR REPLACE FUNCTION content.func_load_posts(
    p_user_id BIGINT DEFAULT NULL,            -- filtrer sur un utilisateur spécifique (NULL = tous)
    p_post_ids BIGINT[] DEFAULT NULL,         -- liste d'IDs de posts à charger (NULL = aucun filtre)
    p_visibility SMALLINT[] DEFAULT ARRAY[0,1], -- visibilités autorisées (0=public,1=amis)
    p_order_mode SMALLINT DEFAULT 0         -- 0=plus récents, 1=plus anciens
)
RETURNS TABLE(
    id BIGINT,
    user_id BIGINT,
    content TEXT,
    hashtags TEXT[],
    identifiers BIGINT[],
    media_ids BIGINT[],
    visibility SMALLINT,
    location TEXT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    like_count INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        p.id,
        p.user_id,
        p.content,
        p.hashtags,
        p.identifiers,
        p.media_ids,
        p.visibility,
        p.location,
        p.created_at,
        p.updated_at,
        COALESCE(
            (SELECT COUNT(*) 
             FROM content.likes l 
             WHERE l.target_type = 0 
               AND l.target_id = p.id),
        0) AS like_count
    FROM content.posts p
    WHERE
        p.visibility != 2  -- ne pas charger les posts supprimés
        AND (p_user_id IS NULL OR p.user_id = p_user_id)
        AND (p_post_ids IS NULL OR p.id = ANY(p_post_ids))
        AND (p.visibility = ANY(p_visibility))
    ORDER BY
        CASE WHEN p_order_mode = 0 THEN p.created_at END DESC,  -- plus récents
        CASE WHEN p_order_mode = 1 THEN p.created_at END ASC;   -- plus anciens
END;
$$;