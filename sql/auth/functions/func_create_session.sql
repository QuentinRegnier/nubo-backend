CREATE OR REPLACE FUNCTION auth.func_create_session(
    p_user_id BIGINT,
    p_refresh_token TEXT,
    p_device_token TEXT,
    p_device_info JSONB,
    p_ip_history INET[],
    p_expires_at TIMESTAMPTZ,
    OUT v_session_id BIGINT,
    OUT v_created_at TIMESTAMPTZ
) AS $$
BEGIN
    INSERT INTO auth.sessions (
        user_id, refresh_token, device_token, device_info, ip_history, expires_at
    ) VALUES (
        p_user_id, p_refresh_token, p_device_token, p_device_info, p_ip_history, p_expires_at
    )
    RETURNING id, created_at INTO v_session_id, v_created_at;
END;
$$ LANGUAGE plpgsql;