CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique de la conversation
    type SMALLINT, -- type de la conversation (0 = message privée, 1 = groupe, 2 = communauté, 3 = annonce)
    title TEXT DEFAULT NULL, -- titre de la conversation
    last_message_id UUID UNIQUE DEFAULT NULL, -- id du dernier message
    last_read_by_all_message_id UUID DEFAULT NULL, -- id du dernier message lu par tous
    state SMALLINT DEFAULT 0, -- état de la conversation (0 = active, 1 = supprimée, 2 = archivée)
    laws SMALLINT[], -- lois applicables à la conversation
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_conversations_last_message ON conversations(last_message_id);