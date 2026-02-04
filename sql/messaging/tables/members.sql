CREATE TABLE IF NOT EXISTS members (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    conversation_id BIGINT REFERENCES conversations(id), -- Corrigé : pointe vers 'conversations'
    user_id BIGINT REFERENCES auth.users(id), -- Harmonisé vers 'users'
    role SMALLINT,                      -- Modifié : Plus de valeur par défaut (0)
    joined_at TIMESTAMPTZ,              -- Modifié : Plus de DEFAULT now()
    unread_count INT,                   -- Modifié : Plus de valeur par défaut (0)
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    UNIQUE(conversation_id, user_id)    -- Note : Virgule ajoutée ici (absente dans l'original)
);

CREATE INDEX idx_members_conversation ON members(conversation_id);
CREATE INDEX idx_members_user ON members(user_id);