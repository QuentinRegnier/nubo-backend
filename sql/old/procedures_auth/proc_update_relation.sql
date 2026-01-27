CREATE OR REPLACE PROCEDURE proc_update_relation(
    p_primary_id BIGINT,
    p_secondary_id BIGINT,
    p_state SMALLINT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE auth.relations
    SET state = COALESCE(p_state, state)
    WHERE primary_id = p_primary_id 
      AND secondary_id = p_secondary_id;
END;
$$;