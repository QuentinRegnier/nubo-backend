CREATE OR REPLACE PROCEDURE proc_update_user(
    p_user_id BIGINT,
    p_username TEXT,
    p_email TEXT,
    p_email_verified BOOLEAN,
    p_phone TEXT,
    p_phone_verified BOOLEAN,
    p_password_hash TEXT,
    p_first_name TEXT DEFAULT NULL,
    p_last_name TEXT DEFAULT NULL,
    p_profile_picture_id BIGINT DEFAULT NULL,
    location TEXT DEFAULT NULL,
    school TEXT DEFAULT NULL,
    work TEXT DEFAULT NULL,
    desactivated BOOLEAN DEFAULT NULL,
    updated_at TIMESTAMPTZ DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE users
    SET email = COALESCE(p_email, email),
        phone = COALESCE(p_phone, phone),
        password_hash = COALESCE(p_password_hash, password_hash),
        updated_at = now()
    WHERE id = p_user_id;
END;
$$;