CREATE TABLE IF NOT EXISTS likes (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    target_type SMALLINT NOT NULL,
    target_id BIGINT NOT NULL,
    user_id BIGINT REFERENCES users(id), -- Harmonisé vers 'users'
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    UNIQUE(target_type, target_id, user_id)
);

CREATE INDEX idx_likes_target ON likes(target_type, target_id);