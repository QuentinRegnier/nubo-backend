CREATE OR REPLACE PROCEDURE proc_create_report(
    p_actor_id BIGINT,
    p_target_type SMALLINT,
    p_target_id BIGINT,
    p_reason TEXT,
    p_state SMALLINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO moderation.reports(actor_id, target_type, target_id, reason, rationale, state)
    VALUES (p_actor_id, p_target_type, p_target_id, p_reason, NULL, p_state);
END;
$$;