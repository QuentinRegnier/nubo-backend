CREATE OR REPLACE FUNCTION auth.func_load_user_settings(
    p_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
RETURNS TABLE (
    id BIGINT,
    user_id BIGINT,
    privacy JSONB,
    notifications JSONB,
    language TEXT,
    theme SMALLINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        s.id,
        s.user_id,
        s.privacy,
        s.notifications,
        s.language,
        s.theme
    FROM auth.user_settings AS s
    WHERE
        (p_id IS NULL OR s.id = p_id)
        AND (p_user_id IS NULL OR s.user_id = p_user_id);
END;
$$ LANGUAGE plpgsql STABLE;