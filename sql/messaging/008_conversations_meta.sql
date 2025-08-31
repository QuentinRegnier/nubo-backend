CREATE TABLE IF NOT EXISTS conversations_meta (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique de la conversation
    type SMALLINT, -- type de la conversation (0 = message privée, 1 = groupe, 2 = communauté, 3 = annonce)
    title TEXT, -- titre de la conversation
    last_message_id UUID UNIQUE, -- id du dernier message
    state SMALLINT DEFAULT 0, -- état de la conversation (0 = active, 1 = supprimée, 2 = archivée)
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_conversations_last_message ON conversations_meta(last_message_id);
