CREATE OR REPLACE PROCEDURE proc_add_like(
    p_target_type SMALLINT,
    p_target_id UUID,
    p_user_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO content.likes(target_type, target_id, user_id)
    VALUES (p_target_type, p_target_id, p_user_id)
    ON CONFLICT (target_type, target_id, user_id) DO NOTHING;
END;
$$;