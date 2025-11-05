CREATE OR REPLACE PROCEDURE proc_delete_media(
    p_id UUID DEFAULT NULL,
    p_owner_id UUID DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_owner_id IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to disable a media';
    END IF;

    UPDATE media
    SET visibility = FALSE
    WHERE (id = p_id OR p_id IS NULL)
      AND (owner_id = p_owner_id OR p_owner_id IS NULL);
END;
$$;