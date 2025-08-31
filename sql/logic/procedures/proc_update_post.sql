CREATE OR REPLACE PROCEDURE proc_update_post(
    p_post_id UUID,
    p_user_id UUID,
    p_content TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE content.posts
    SET content = p_content,
        updated_at = now()
    WHERE id = p_post_id AND user_id = p_user_id;
END;
$$;
