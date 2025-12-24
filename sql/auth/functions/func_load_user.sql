CREATE OR REPLACE FUNCTION auth.func_load_user(
    p_id BIGINT DEFAULT NULL,
    p_username TEXT DEFAULT NULL,
    p_email TEXT DEFAULT NULL,
    p_phone TEXT DEFAULT NULL
)
RETURNS TABLE (
    id BIGINT,
    username TEXT,
    email TEXT,
    email_verified BOOLEAN,
    phone TEXT,
    phone_verified BOOLEAN,
    password_hash TEXT,
    first_name TEXT,
    last_name TEXT,
    birthdate DATE,
    sex SMALLINT,
    bio TEXT,
    profile_picture_id BIGINT,
    grade SMALLINT,
    location TEXT,
    school TEXT,
    work TEXT,
    badges TEXT[],
    desactivated BOOLEAN,
    banned BOOLEAN,
    ban_reason TEXT,
    ban_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        u.id,
        u.username,
        u.email,
        u.email_verified,
        u.phone,
        u.phone_verified,
        u.password_hash,
        u.first_name,
        u.last_name,
        u.birthdate,
        u.sex,
        u.bio,
        u.profile_picture_id,
        u.grade,
        u.location,
        u.school,
        u.work,
        u.badges,
        u.desactivated,
        u.banned,
        u.ban_reason,
        u.ban_expires_at,
        u.created_at,
        u.updated_at
    FROM auth.users AS u
    WHERE
        (p_id IS NULL OR u.id = p_id)
        AND (p_username IS NULL OR TRIM(u.username) = TRIM(p_username))
        -- MODIFICATION ICI : On nettoie les espaces (TRIM) et on ignore la casse (LOWER)
        AND (p_email IS NULL OR TRIM(LOWER(u.email)) = TRIM(LOWER(p_email)))
        AND (p_phone IS NULL OR TRIM(u.phone) = TRIM(p_phone));
END;
$$ LANGUAGE plpgsql STABLE;