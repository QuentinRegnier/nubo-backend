CREATE TABLE IF NOT EXISTS relations (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    primary_id BIGINT REFERENCES users(id),
    secondary_id BIGINT REFERENCES users(id),
    state SMALLINT,                     -- Modifié : Plus de valeur par défaut (1)
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    UNIQUE(secondary_id, primary_id)
);

CREATE INDEX idx_relations_primary_id ON relations(primary_id);
CREATE INDEX idx_relations_secondary_id ON relations(secondary_id);