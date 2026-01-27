CREATE TABLE IF NOT EXISTS comments (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    post_id BIGINT REFERENCES posts(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id), -- Harmonisé vers 'users'
    content TEXT,
    visibility BOOLEAN,                 -- Modifié : Plus de valeur par défaut (TRUE)
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);

CREATE INDEX idx_comments_post_created ON comments(post_id, created_at DESC);