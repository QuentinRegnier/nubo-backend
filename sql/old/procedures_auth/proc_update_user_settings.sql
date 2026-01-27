CREATE OR REPLACE PROCEDURE proc_update_user_settings(
    p_id BIGINT,
    p_user_id BIGINT,
    p_privacy JSONB DEFAULT NULL,
    p_notifications JSONB DEFAULT NULL,
    p_language TEXT DEFAULT NULL,
    p_theme TEXT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE user_settings
    SET 
        privacy           = COALESCE(p_privacy, privacy),
        notifications     = COALESCE(p_notifications, notifications),
        language          = COALESCE(p_language, language),
        theme             = COALESCE(p_theme, theme)
    WHERE (id = p_id OR p_id IS NULL)
      AND (user_id = p_user_id OR p_user_id IS NULL);
END;
$$;