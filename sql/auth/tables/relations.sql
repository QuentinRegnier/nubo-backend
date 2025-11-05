CREATE TABLE IF NOT EXISTS relations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du suivi
    primary_id UUID REFERENCES users(id), -- id de l'utilisateur qui suit
    secondary_id UUID REFERENCES users(id), -- id de l'utilisateur suivi
    state SMALLINT DEFAULT 1, -- état du suivi (2 = amis, 1 = suivi, 0 = inactif, -1 = bloqué)
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    UNIQUE(secondary_id, primary_id)
);

CREATE INDEX idx_relations_primary_id ON relations(primary_id);
CREATE INDEX idx_relations_secondary_id ON relations(secondary_id);