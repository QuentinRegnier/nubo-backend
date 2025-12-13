CREATE TABLE IF NOT EXISTS members (
    id BIGSERIAL PRIMARY KEY, -- id unique du membre
    conversation_id BIGINT REFERENCES conversations_meta(id), -- id de la conversation
    user_id BIGINT REFERENCES auth.users(id), -- id de l'utilisateur
    role SMALLINT DEFAULT 0, -- rôle du membre (0 = membre, 1 = admin, 2 = créateur)
    joined_at TIMESTAMPTZ DEFAULT now(), -- date d'adhésion
    unread_count INT DEFAULT 0, -- nombre de messages non lus
    UNIQUE(conversation_id, user_id)
);