CREATE OR REPLACE PROCEDURE auth.proc_update_session(
    p_id BIGINT,
    p_refresh_token TEXT DEFAULT NULL,
    p_device_info JSONB DEFAULT NULL,
    p_device_token TEXT DEFAULT NULL,
    p_ip_history INET[] DEFAULT NULL,
    p_expires_at TIMESTAMPTZ DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE auth.sessions
    SET
        refresh_token = COALESCE(p_refresh_token, refresh_token),
        device_info   = COALESCE(p_device_info, device_info),
        device_token  = COALESCE(p_device_token, device_token),
        ip_history    = COALESCE(p_ip_history, ip_history),
        expires_at    = COALESCE(p_expires_at, expires_at)
    WHERE id = p_id;
END;
$$;