CREATE OR REPLACE FUNCTION auth.func_create_user(
    p_username TEXT,
    p_email TEXT,
    p_phone TEXT,
    p_password_hash TEXT,
    p_first_name TEXT,
    p_last_name TEXT,
    p_birthdate DATE,
    p_sex SMALLINT,
    p_bio TEXT,
    p_profile_picture_id UUID,
    p_location TEXT,
    p_school TEXT,
    p_work TEXT,
    p_badges TEXT[],
    p_privacy JSONB,
    p_notifications JSONB,
    p_language TEXT,
    p_theme TEXT,
    p_refresh_token TEXT,
    p_device_info JSONB,
    p_ip INET[],
    p_expires_at TIMESTAMPTZ
) RETURNS UUID AS $$
DECLARE
    v_user_id UUID;
BEGIN
    -- 1️⃣ Créer l'utilisateur
    INSERT INTO auth.users (
        username, email, phone, password_hash, first_name, last_name,
        birthdate, sex, bio, profile_picture_id, location, school, work, badges
    ) VALUES (
        p_username, p_email, p_phone, p_password_hash, p_first_name, p_last_name,
        p_birthdate, p_sex, p_bio, p_profile_picture_id, p_location, p_school, p_work, p_badges
    )
    RETURNING id INTO v_user_id;

    -- 2️⃣ Créer les settings associés
    INSERT INTO auth.user_settings (user_id, privacy, notifications, language, theme)
    VALUES (v_user_id, p_privacy, p_notifications, p_language, p_theme);

    -- 3️⃣ Créer la session
    INSERT INTO auth.sessions (user_id, refresh_token, device_info, ip, expires_at)
    VALUES (v_user_id, p_refresh_token, p_device_info, p_ip, p_expires_at);

    RETURN v_user_id;
END;
$$ LANGUAGE plpgsql;
