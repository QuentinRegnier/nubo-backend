CREATE OR REPLACE FUNCTION content.func_load_comments(
    p_post_id UUID DEFAULT NULL,          -- filtrer sur un post spécifique (NULL = tous les posts)
    p_user_id UUID DEFAULT NULL,          -- filtrer sur un utilisateur spécifique (NULL = tous)
    p_limit INT DEFAULT 100,              -- limite de résultats
    p_order_mode SMALLINT DEFAULT 0       -- 0=plus récents, 1=plus likés, 2=plus anciens
)
RETURNS TABLE(
    id UUID,
    content TEXT,
    user_id UUID,
    post_id UUID,
    created_at TIMESTAMPTZ,
    like_count INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        c.id,
        c.content,
        c.user_id,
        c.post_id,
        c.created_at,
        COALESCE(
            (SELECT COUNT(*) 
             FROM content.likes l 
             WHERE l.target_type = 2 
               AND l.target_id = c.id),
        0) AS like_count
    FROM content.comments c
    WHERE
        c.visibility = TRUE  -- ✅ on ignore les commentaires invisibles
        AND (p_post_id IS NULL OR c.post_id = p_post_id)
        AND (p_user_id IS NULL OR c.user_id = p_user_id)
    ORDER BY
        CASE WHEN p_order_mode = 0 THEN c.created_at END DESC,   -- plus récents
        CASE WHEN p_order_mode = 1 THEN like_count END DESC,      -- plus likés
        CASE WHEN p_order_mode = 2 THEN c.created_at END ASC      -- plus anciens
    LIMIT p_limit;
END;
$$;