CREATE OR REPLACE FUNCTION moderation.func_load_reports_by_state(
    p_state SMALLINT,
    p_limit INT
)
-- 1. Définit la structure de retour (correspond à la table 'reports')
RETURNS TABLE (
    id UUID,
    actor_id UUID,
    target_type SMALLINT,
    target_id UUID,
    reason TEXT,
    rationale TEXT DEFAULT NULL,
    state SMALLINT,
    created_at TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- 2. Retourne le résultat de la requête
    RETURN QUERY
    SELECT
        r.id,
        r.actor_id,
        r.target_type,
        r.target_id,
        r.reason,
        r.rationale,
        r.state,
        r.created_at
    FROM
        moderation.reports AS r
    WHERE
        -- 3. Filtre par l'état demandé
        r.state = p_state
    ORDER BY
        -- 4. Trie par date de création (les "premiers" = les plus anciens)
        r.created_at ASC
    LIMIT
        -- 5. Limite au nombre N demandé
        p_limit;
END;
$$;