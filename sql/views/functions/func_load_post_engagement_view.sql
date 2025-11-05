CREATE OR REPLACE FUNCTION views.func_load_post_engagement_view(
    p_post_id UUID DEFAULT NULL,
    p_user_id UUID DEFAULT NULL
)
RETURNS TABLE (
    post_id UUID,
    user_id UUID,
    content TEXT,
    media_ids UUID[],
    visibility SMALLINT,
    location TEXT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    like_count BIGINT,
    comment_count BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT *
    FROM public.post_engagement_view
    WHERE 
        (p_post_id IS NULL OR post_engagement_view.post_id = p_post_id)
        AND (p_user_id IS NULL OR post_engagement_view.user_id = p_user_id);
END;
$$;