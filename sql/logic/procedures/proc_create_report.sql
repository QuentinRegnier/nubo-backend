CREATE OR REPLACE PROCEDURE proc_create_report(
    p_actor_id UUID,
    p_target_type SMALLINT,
    p_target_id UUID,
    p_reason TEXT,
    p_state SMALLINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO moderation.reports(actor_id, target_type, target_id, reason, state)
    VALUES (p_actor_id, p_target_type, p_target_id, p_reason, p_state);
END;
$$;
