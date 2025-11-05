CREATE OR REPLACE FUNCTION auth.func_load_sessions(
    p_id UUID DEFAULT NULL,
    p_user_id UUID DEFAULT NULL,
    p_device_token TEXT DEFAULT NULL,
    p_refresh_token TEXT DEFAULT NULL
)
RETURNS TABLE (
    id UUID,
    user_id UUID,
    refresh_token TEXT,
    device_token TEXT,
    device_info JSONB,
    ip_history INET[],
    created_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        s.id,
        s.user_id,
        s.refresh_token,
        s.device_token,
        s.device_info,
        s.ip_history,
        s.created_at,
        s.expires_at
    FROM auth.sessions AS s
    WHERE
        (p_id IS NULL OR s.id = p_id)
        AND (p_user_id IS NULL OR s.user_id = p_user_id)
        AND (p_device_token IS NULL OR s.device_token = p_device_token)
        AND (p_refresh_token IS NULL OR s.refresh_token = p_refresh_token);
END;
$$ LANGUAGE plpgsql STABLE;