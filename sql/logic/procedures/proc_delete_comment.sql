CREATE OR REPLACE PROCEDURE proc_delete_comment(
    p_comment_id UUID,
    p_user_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM content.comments
    WHERE id = p_comment_id AND user_id = p_user_id;
END;
$$;
