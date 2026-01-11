CREATE TABLE IF NOT EXISTS comments (
    id BIGSERIAL PRIMARY KEY, -- id unique du commentaire
    post_id BIGINT REFERENCES posts(id) ON DELETE CASCADE, -- id du post
    user_id BIGINT REFERENCES auth.users(id), -- id de l'utilisateur
    content TEXT, -- contenu du commentaire
    visibility BOOLEAN DEFAULT TRUE, -- visibilité du commentaire
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    updated_at TIMESTAMPTZ DEFAULT now() -- date de mise à jour
);

CREATE INDEX idx_comments_post_created ON comments(post_id, created_at DESC);