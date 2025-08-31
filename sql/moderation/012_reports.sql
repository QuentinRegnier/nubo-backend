CREATE TABLE IF NOT EXISTS reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du rapport
    actor_id UUID REFERENCES auth.users(id), -- id de l'utilisateur ayant signalé
    target_type SMALLINT NOT NULL, -- type de la cible (user/post/comment/etc)
    target_id UUID NOT NULL, -- id de la cible
    reason TEXT, -- raison du signalement
    state SMALLINT DEFAULT 0, -- état du rapport (0=pending, 1=reviewed, 2=resolved)
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_reports_actor ON reports(actor_id);
CREATE INDEX idx_reports_created ON reports(created_at);
