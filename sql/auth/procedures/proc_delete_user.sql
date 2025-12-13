CREATE OR REPLACE PROCEDURE proc_delete_user(
    p_id BIGINT DEFAULT NULL,
    p_username TEXT DEFAULT NULL,
    p_email TEXT DEFAULT NULL,
    p_phone TEXT DEFAULT NULL,
    p_ban BOOLEAN DEFAULT FALSE,
    p_ban_reason TEXT DEFAULT NULL,
    p_ban_expires_at TIMESTAMPTZ DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_username IS NULL AND p_email IS NULL AND p_phone IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to deactivate or ban a user';
    END IF;

    UPDATE users
    SET desactivated = TRUE,
        banned = p_ban,
        ban_reason = CASE WHEN p_ban THEN p_ban_reason ELSE NULL END,
        ban_expires_at = CASE WHEN p_ban THEN p_ban_expires_at ELSE NULL END,
        updated_at = now()
    WHERE (id = p_id OR p_id IS NULL)
      AND (username = p_username OR p_username IS NULL)
      AND (email = p_email OR p_email IS NULL)
      AND (phone = p_phone OR p_phone IS NULL);
END;
$$;