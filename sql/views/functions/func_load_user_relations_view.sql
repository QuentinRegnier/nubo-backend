CREATE OR REPLACE FUNCTION views.func_load_user_relations_view(
    p_follower_id UUID DEFAULT NULL,
    p_followed_id UUID DEFAULT NULL
)
RETURNS TABLE (
    relation_id UUID,
    follower_id UUID,
    follower_username TEXT,
    followed_id UUID,
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