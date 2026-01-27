CREATE TABLE IF NOT EXISTS messages (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    conversation_id BIGINT REFERENCES conversations(id), -- Corrigé : pointe vers 'conversations'
    sender_id BIGINT NOT NULL,          -- Note : Pas de FK explicite ici dans l'original, gardé tel quel
    message_type SMALLINT NOT NULL,     -- Modifié : Plus de valeur par défaut (0)
    visibility BOOLEAN,                 -- Modifié : Plus de valeur par défaut (TRUE)
    content TEXT,
    attachments JSONB,
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);

CREATE INDEX idx_message_conv_created ON messages(conversation_id, created_at DESC);