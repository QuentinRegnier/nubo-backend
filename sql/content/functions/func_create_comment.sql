CREATE OR REPLACE FUNCTION content.func_create_comment(
    p_post_id BIGINT,
    p_user_id BIGINT,
    p_content TEXT
) RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
    v_comment_id BIGINT;
BEGIN
    INSERT INTO content.comments(post_id, user_id, content)
    VALUES (p_post_id, p_user_id, p_content)
    RETURNING id INTO v_comment_id;

    RETURN v_comment_id;
END;
$$;