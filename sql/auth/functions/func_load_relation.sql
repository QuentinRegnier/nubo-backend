CREATE OR REPLACE FUNCTION auth.func_load_relations(
    p_id UUID DEFAULT NULL,
    p_primary_id UUID DEFAULT NULL,
    p_secondary_id UUID DEFAULT NULL,
    p_state SMALLINT DEFAULT NULL
)
RETURNS TABLE (
    id UUID,
    primary_id UUID,
    secondary_id UUID,
    state SMALLINT,
    created_at TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        r.id,
        r.primary_id,
        r.secondary_id,
        r.state,
        r.created_at
    FROM auth.relations AS r
    WHERE
        (p_id IS NULL OR r.id = p_id)
        AND (p_primary_id IS NULL OR r.primary_id = p_primary_id)
        AND (p_secondary_id IS NULL OR r.secondary_id = p_secondary_id)
        AND (p_state IS NULL OR r.state = p_state);
END;
$$ LANGUAGE plpgsql STABLE;