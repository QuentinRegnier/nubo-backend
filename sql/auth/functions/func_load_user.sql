CREATE OR REPLACE FUNCTION auth.func_load_user(
    p_id UUID DEFAULT NULL,
    p_username TEXT DEFAULT NULL,
    p_email TEXT DEFAULT NULL,
    p_phone TEXT DEFAULT NULL
)
RETURNS TABLE (
    id UUID,
    username TEXT,
    email TEXT,
    email_verified BOOLEAN,
    phone TEXT,
    phone_verified BOOLEAN,
    first_name TEXT,
    last_name TEXT,
    birthdate DATE,
    sex SMALLINT,
    bio TEXT,
    profile_picture_id UUID,
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
        AND (p_username IS NULL OR u.username = p_username)
        AND (p_email IS NULL OR u.email = p_email)
        AND (p_phone IS NULL OR u.phone = p_phone);
END;
$$ LANGUAGE plpgsql STABLE;