CREATE TABLE IF NOT EXISTS posts (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    user_id BIGINT REFERENCES auth.users(id) NOT NULL, -- Harmonisé vers 'users'
    content TEXT,
    hashtags TEXT[],
    identifiers BIGINT[],
    media_ids BIGINT[],
    visibility SMALLINT,                -- Modifié : Plus de valeur par défaut (0)
    location TEXT,
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);

CREATE INDEX idx_posts_user_created ON posts(user_id, created_at DESC);