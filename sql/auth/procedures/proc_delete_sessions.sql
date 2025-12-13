CREATE OR REPLACE PROCEDURE proc_delete_session(
    p_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL,
    p_device_token TEXT DEFAULT NULL,
    p_refresh_token TEXT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_user_id IS NULL AND p_device_token IS NULL AND p_refresh_token IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to delete a session';
    END IF;

    DELETE FROM sessions
    WHERE (id = p_id OR p_id IS NULL)
      AND (user_id = p_user_id OR p_user_id IS NULL)
      AND (device_token = p_device_token OR p_device_token IS NULL)
      AND (refresh_token = p_refresh_token OR p_refresh_token IS NULL);
END;
$$;