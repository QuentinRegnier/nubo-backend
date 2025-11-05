CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du message
    conversation_id UUID REFERENCES conversations_meta(id), -- id de la conversation
    sender_id UUID NOT NULL, -- id de l'expéditeur
    message_type SMALLINT NOT NULL DEFAULT 0, -- 0=text, 1=image, 2=publication, 3=vocal, 4=vidéo
    visibility BOOLEAN DEFAULT TRUE, -- true si le message est visible par les membres (ou supprimé par l'expéditeur)
    content TEXT, -- contenu du message
    attachments JSONB, -- pointeurs vers fichiers S3 / metadata
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_message_conv_created ON messages(conversation_id, created_at DESC);