CREATE OR REPLACE PROCEDURE proc_delete_comment(
    p_id BIGINT DEFAULT NULL,
    p_post_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_post_id IS NULL AND p_user_id IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to hide a comment';
    END IF;

    UPDATE comments
    SET visibility = FALSE
    WHERE (id = p_id OR p_id IS NULL)
      AND (post_id = p_post_id OR p_post_id IS NULL)
      AND (user_id = p_user_id OR p_user_id IS NULL);
END;
$$;