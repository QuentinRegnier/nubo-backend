CREATE TABLE IF NOT EXISTS media (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    owner_id BIGINT REFERENCES users(id), -- Harmonisé vers 'users'
    storage_path TEXT,
    visibility BOOLEAN,                 -- Modifié : Plus de valeur par défaut (TRUE)
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);

CREATE INDEX idx_media_owner ON media(owner_id);
CREATE INDEX idx_media_created ON media(created_at);