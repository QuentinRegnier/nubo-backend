CREATE TABLE IF NOT EXISTS media (
    id BIGSERIAL PRIMARY KEY, -- id unique du média
    owner_id BIGINT REFERENCES auth.users(id), -- id du propriétaire
    storage_path TEXT, -- chemin de stockage
    visibility BOOLEAN DEFAULT TRUE, -- true si le media est utilisé dans un post/un message/une image de profil publique
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    updated_at TIMESTAMPTZ DEFAULT now() -- date de mise à jour
);

CREATE INDEX idx_media_owner ON media(owner_id);
CREATE INDEX idx_media_created ON media(created_at);