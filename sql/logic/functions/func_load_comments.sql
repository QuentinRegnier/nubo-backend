CREATE OR REPLACE FUNCTION content.func_load_comments(
    p_post_id UUID,
    p_limit INT,
    p_order_mode SMALLINT DEFAULT 0 -- 0=recent, 1=most liked
)
RETURNS TABLE(
    id UUID,
    content TEXT,
    user_id UUID,
    created_at TIMESTAMPTZ,
    like_count INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT c.id, c.content, c.user_id, c.created_at,
           (SELECT COUNT(*) FROM content.likes l WHERE l.target_type=2 AND l.target_id=c.id) AS like_count
    FROM content.comments c
    WHERE c.post_id = p_post_id
    ORDER BY
        CASE WHEN p_order_mode=0 THEN c.created_at END DESC,
        CASE WHEN p_order_mode=1 THEN (SELECT COUNT(*) FROM content.likes l WHERE l.target_type=2 AND l.target_id=c.id) END DESC
    LIMIT p_limit;
END;
$$;
