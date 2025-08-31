CREATE OR REPLACE PROCEDURE proc_update_relation(
    p_primary_id UUID,
    p_secondary_id UUID,
    p_state SMALLINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE auth.relations
    SET state = p_state
    WHERE primary_id = p_primary_id AND secondary_id = p_secondary_id;
END;
$$;
