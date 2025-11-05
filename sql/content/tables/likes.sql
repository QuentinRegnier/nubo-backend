CREATE TABLE IF NOT EXISTS likes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du like
    target_type SMALLINT NOT NULL, -- type de la cible (0 = post, 1 = message, 2 = commentaire)
    target_id UUID NOT NULL, -- id de la cible
    user_id UUID REFERENCES auth.users(id), -- id de l'utilisateur
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    UNIQUE(target_type, target_id, user_id)
);

CREATE INDEX idx_likes_target ON likes(target_type, target_id);