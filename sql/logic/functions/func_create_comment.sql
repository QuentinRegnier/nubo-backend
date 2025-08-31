CREATE OR REPLACE FUNCTION func_create_comment(
    p_post_id UUID,
    p_user_id UUID,
    p_content TEXT
) RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_comment_id UUID;
BEGIN
    INSERT INTO content.comments(post_id, user_id, content)
    VALUES (p_post_id, p_user_id, p_content)
    RETURNING id INTO v_comment_id;

    RETURN v_comment_id;
END;
$$;