CREATE OR REPLACE FUNCTION content.func_add_like(
    p_target_type SMALLINT,
    p_target_id BIGINT,
    p_user_id BIGINT
) RETURNS BIGINT AS $$
DECLARE
    v_like_id BIGINT;
BEGIN
    INSERT INTO content.likes(target_type, target_id, user_id)
    VALUES (p_target_type, p_target_id, p_user_id)
    ON CONFLICT (target_type, target_id, user_id) DO NOTHING
    RETURNING id INTO v_like_id;

    -- Si le like existait déjà, on récupère son id
    IF v_like_id IS NULL THEN
        SELECT id INTO v_like_id
        FROM content.likes
        WHERE target_type = p_target_type
          AND target_id = p_target_id
          AND user_id = p_user_id;
    END IF;

    RETURN v_like_id;
END;
$$ LANGUAGE plpgsql;