CREATE OR REPLACE FUNCTION auth.func_create_relation(
    p_primary_id BIGINT,
    p_secondary_id BIGINT,
    p_state SMALLINT DEFAULT 1
) RETURNS BIGINT AS $$
DECLARE
    v_relation_id BIGINT;
BEGIN
    INSERT INTO auth.relations(primary_id, secondary_id, state)
    VALUES (p_primary_id, p_secondary_id, p_state)
    ON CONFLICT (primary_id, secondary_id) DO UPDATE
    SET state = excluded.state
    RETURNING id INTO v_relation_id;

    RETURN v_relation_id;
END;
$$ LANGUAGE plpgsql;