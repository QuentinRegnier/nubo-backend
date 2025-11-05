CREATE TABLE IF NOT EXISTS posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du post
    user_id UUID REFERENCES auth.users(id) NOT NULL, -- id de l'utilisateur
    content TEXT, -- contenu du post
    media_ids UUID[], -- ids des médias associés
    visibility SMALLINT DEFAULT 0, -- visibilité (2= supprimer, 1 = amis, 0 = public)
    location TEXT, -- localisation
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    updated_at TIMESTAMPTZ DEFAULT now() -- date de mise à jour
);

CREATE INDEX idx_posts_user_created ON posts(user_id, created_at DESC);