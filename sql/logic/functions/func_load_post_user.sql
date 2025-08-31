CREATE OR REPLACE FUNCTION func_get_user_posts(
    p_user_id UUID
) RETURNS SETOF content.posts
LANGUAGE sql
AS $$
    SELECT *
    FROM content.posts
    WHERE user_id = p_user_id
    ORDER BY created_at DESC;
$$;