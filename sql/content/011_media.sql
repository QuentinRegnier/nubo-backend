CREATE TABLE IF NOT EXISTS media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du média
    owner_id UUID REFERENCES auth.users(id), -- id du propriétaire
    storage_path TEXT, -- chemin de stockage
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_media_owner ON media(owner_id);
CREATE INDEX idx_media_created ON media(created_at);