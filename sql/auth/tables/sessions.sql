CREATE TABLE IF NOT EXISTS sessions (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    user_id BIGINT REFERENCES users(id) NOT NULL,
    master_token TEXT NOT NULL,
    device_token TEXT NOT NULL,
    device_info JSONB,
    ip_history INET[],
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    expires_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_sessions_user_device 
  ON sessions(user_id, device_token);