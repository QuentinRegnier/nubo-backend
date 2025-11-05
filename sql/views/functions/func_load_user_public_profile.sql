CREATE OR REPLACE FUNCTION views.func_load_user_public_profile(
    p_user_id UUID DEFAULT NULL
)
RETURNS TABLE (
    user_id UUID,
    username TEXT,
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
    privacy JSONB, -- En supposant que 'privacy' est un type JSONB dans user_settings
    notifications JSONB, -- En supposant que 'notifications' est un type JSONB
    language TEXT,
    theme TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Note : J'ai dû deviner les types pour s.privacy, s.notifications, etc.
    -- Ajustez-les (ex: JSONB, TEXT) si nécessaire.
    RETURN QUERY
    SELECT *
    FROM public.user_public_profile
    WHERE 
        (p_user_id IS NULL OR user_public_profile.user_id = p_user_id);
END;
$$;