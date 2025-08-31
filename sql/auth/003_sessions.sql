CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique de la session
    user_id UUID REFERENCES users(id), -- id de l'utilisateur
    refresh_token TEXT, -- token de rafraîchissement
    device_info JSONB, -- informations sur l'/les appareil(s)
    ip INET[], -- adresse IP
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    expires_at TIMESTAMPTZ, -- date d'expiration
    revoked BOOLEAN DEFAULT FALSE -- session révoquée
);

CREATE INDEX idx_sessions_user_id_revoked ON sessions(user_id, revoked);
