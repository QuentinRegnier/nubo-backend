CREATE TABLE IF NOT EXISTS conversations (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    type SMALLINT,
    title TEXT,                         -- Modifié : Plus de DEFAULT NULL
    last_message_id BIGINT UNIQUE,      -- Modifié : Plus de DEFAULT NULL
    last_read_by_all_message_id BIGINT, -- Modifié : Plus de DEFAULT NULL
    state SMALLINT,                     -- Modifié : Plus de valeur par défaut (0)
    laws SMALLINT[],
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);

CREATE INDEX idx_conversations_last_message ON conversations(last_message_id);
CREATE INDEX idx_conversations_created ON conversations(created_at DESC);
CREATE INDEX idx_conversations_state ON conversations(state);