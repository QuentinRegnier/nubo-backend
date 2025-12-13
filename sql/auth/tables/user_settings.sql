CREATE TABLE IF NOT EXISTS user_settings (
    id BIGSERIAL PRIMARY KEY, -- id unique des paramètres utilisateur
    user_id BIGINT UNIQUE REFERENCES users(id) ON DELETE CASCADE, -- id unique de l'utilisateur
    privacy JSONB, -- paramètres de confidentialité
    notifications JSONB, -- paramètres de notification
    language TEXT, -- langue
    theme SMALLINT NOT NULL DEFAULT 0 -- thème clair/sombre
);

CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);