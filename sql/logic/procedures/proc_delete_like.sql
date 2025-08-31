CREATE OR REPLACE PROCEDURE proc_remove_like(
    p_target_type SMALLINT,
    p_target_id UUID,
    p_user_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM content.likes
    WHERE target_type = p_target_type
      AND target_id = p_target_id
      AND user_id = p_user_id;
END;
$$;