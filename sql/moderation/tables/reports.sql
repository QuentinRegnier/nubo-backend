CREATE TABLE IF NOT EXISTS reports (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    actor_id BIGINT REFERENCES users(id), -- Harmonisé vers 'users'
    target_type SMALLINT NOT NULL,
    target_id BIGINT NOT NULL,
    reason TEXT,
    rationale TEXT,                     -- Modifié : Plus de DEFAULT NULL
    state SMALLINT,                     -- Modifié : Plus de valeur par défaut (0)
    created_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);

CREATE INDEX idx_reports_actor ON reports(actor_id);
CREATE INDEX idx_reports_created ON reports(created_at);
CREATE INDEX idx_reports_target ON reports(target_type, target_id);
CREATE INDEX idx_reports_state ON reports(state);