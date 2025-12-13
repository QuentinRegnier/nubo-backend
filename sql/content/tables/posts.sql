CREATE TABLE IF NOT EXISTS posts (
    id BIGSERIAL PRIMARY KEY, -- id unique du post
    user_id BIGINT REFERENCES auth.users(id) NOT NULL, -- id de l'utilisateur
    content TEXT, -- contenu du post
    media_ids BIGINT[], -- ids des médias associés
    visibility SMALLINT DEFAULT 0, -- visibilité (2= supprimer, 1 = amis, 0 = public)
    location TEXT, -- localisation
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    updated_at TIMESTAMPTZ DEFAULT now() -- date de mise à jour
);

CREATE INDEX idx_posts_user_created ON posts(user_id, created_at DESC);