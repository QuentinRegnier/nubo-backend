CREATE OR REPLACE PROCEDURE proc_create_relation(
    p_primary_id UUID,
    p_secondary_id UUID,
    p_state SMALLINT DEFAULT 1
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO auth.relations(primary_id, secondary_id, state)
    VALUES (p_primary_id, p_secondary_id, p_state)
    ON CONFLICT (primary_id, secondary_id) DO UPDATE
    SET state = excluded.state;
END;
$$;
