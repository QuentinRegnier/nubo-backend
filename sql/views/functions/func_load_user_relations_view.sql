CREATE OR REPLACE FUNCTION views.func_load_user_relations_view(
    p_follower_id BIGINT DEFAULT NULL,
    p_followed_id BIGINT DEFAULT NULL
)
RETURNS TABLE (
    relation_id BIGINT,
    follower_id BIGINT,
    follower_username TEXT,
    followed_id BIGINT,
    followed_username TEXT,
    state SMALLINT,
    created_at TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT *
    FROM public.user_relations_view
    WHERE 
        (p_follower_id IS NULL OR user_relations_view.follower_id = p_follower_id)
        AND (p_followed_id IS NULL OR user_relations_view.followed_id = p_followed_id);
END;
$$;