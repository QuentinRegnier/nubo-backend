CREATE OR REPLACE FUNCTION auth.func_create_user_settings(
    p_user_id UUID,
    p_privacy JSONB,
    p_notifications JSONB,
    p_language TEXT,
    p_theme SMALLINT DEFAULT 0
) RETURNS UUID AS $$
DECLARE
    v_settings_id UUID;
BEGIN
    INSERT INTO auth.user_settings (
        user_id, privacy, notifications, language, theme
    ) VALUES (
        p_user_id, p_privacy, p_notifications, p_language, p_theme
    )
    RETURNING id INTO v_settings_id;

    RETURN v_settings_id;
END;
$$ LANGUAGE plpgsql;