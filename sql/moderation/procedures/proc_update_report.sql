CREATE OR REPLACE PROCEDURE moderation.proc_update_report(
    p_report_id UUID,
    p_new_state SMALLINT,
    p_new_rationale TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Met à jour le rapport spécifié
    UPDATE moderation.reports
    SET 
        state = p_new_state,
        rationale = p_new_rationale
    WHERE 
        id = p_report_id;
END;
$$;