CREATE OR REPLACE PROCEDURE proc_delete_user_settings(
    p_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_user_id IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to delete a user setting';
    END IF;

    DELETE FROM user_settings
    WHERE (id = p_id OR p_id IS NULL)
      AND (user_id = p_user_id OR p_user_id IS NULL);
END;
$$;