CREATE OR REPLACE PROCEDURE proc_update_comment(
    p_comment_id BIGINT,
    p_user_id BIGINT,
    p_content TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE content.comments
    SET content = p_content
    WHERE id = p_comment_id AND user_id = p_user_id;
END;
$$;