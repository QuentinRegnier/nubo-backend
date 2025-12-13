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
    p_profile_picture_id BIGINT,
    p_grade SMALLINT,
    p_location TEXT,
    p_school TEXT,
    p_work TEXT,
    p_badges TEXT[],
    p_desactivated BOOLEAN,
    p_banned BOOLEAN,
    p_ban_reason TEXT,
    p_ban_expires_at TIMESTAMPTZ,
    p_refresh_token TEXT,
    p_device_info JSONB,
    p_device_token TEXT, 
    p_ip_history INET[],
    p_expires_at TIMESTAMPTZ,
    OUT v_user_id BIGINT,
    OUT v_created_at_user TIMESTAMPTZ,
    OUT v_updated_at_user TIMESTAMPTZ,
    OUT v_session_id BIGINT,
    OUT v_created_at_session TIMESTAMPTZ
) LANGUAGE plpgsql 
AS $$
BEGIN
    -- 1️⃣ Créer l'utilisateur
    INSERT INTO auth.users (
        username, email, phone, password_hash, first_name, last_name,
        birthdate, sex, bio, profile_picture_id, grade, location, school, work, badges,
        desactivated, banned, ban_reason, ban_expires_at
    ) VALUES (
        p_username, p_email, p_phone, p_password_hash, p_first_name, p_last_name,
        p_birthdate, p_sex, p_bio, p_profile_picture_id, p_grade, p_location, p_school, p_work, p_badges,
        p_desactivated, p_banned, p_ban_reason, p_ban_expires_at
    )
    RETURNING id, created_at, updated_at INTO v_user_id, v_created_at_user, v_updated_at_user;

    -- 2️⃣ Créer la session (user_settings supprimé)
    INSERT INTO auth.sessions (user_id, refresh_token, device_info, device_token, ip_history, expires_at)
    VALUES (v_user_id, p_refresh_token, p_device_info, p_device_token::jsonb, p_ip_history, p_expires_at)
    RETURNING id, created_at INTO v_session_id, v_created_at_session;
END;
$$;