CREATE TABLE IF NOT EXISTS comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du commentaire
    post_id UUID REFERENCES posts(id) ON DELETE CASCADE, -- id du post
    user_id UUID REFERENCES auth.users(id), -- id de l'utilisateur
    content TEXT, -- contenu du commentaire
    visibility BOOLEAN DEFAULT TRUE, -- visibilité du commentaire
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_comments_post_created ON comments(post_id, created_at DESC);