CREATE TABLE IF NOT EXISTS user_settings (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    user_id BIGINT UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    privacy JSONB,
    notifications JSONB,
    language TEXT,
    theme SMALLINT NOT NULL,            -- Modifié : Plus de valeur par défaut
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);

CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);