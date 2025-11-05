CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),        -- id unique de la session
    user_id UUID REFERENCES users(id) NOT NULL,          -- id de l'utilisateur
    refresh_token TEXT NOT NULL,                          -- token de rafraîchissement
    device_token TEXT NOT NULL,                           -- identifiant unique de l'appareil
    device_info JSONB,                                    -- informations sur l'appareil (OS, modèle, version...)
    ip_history INET[],                                    -- historique des IP utilisées
    created_at TIMESTAMPTZ DEFAULT now(),                -- date de création
    expires_at TIMESTAMPTZ                                -- date d'expiration
);

CREATE UNIQUE INDEX idx_sessions_user_device 
  ON sessions(user_id, device_token);