CREATE OR REPLACE PROCEDURE proc_delete_post(
    p_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_user_id IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to hide a post';
    END IF;

    UPDATE posts
    SET visibility = 2,
        updated_at = now()
    WHERE (id = p_id OR p_id IS NULL)
      AND (user_id = p_user_id OR p_user_id IS NULL);
END;
$$;