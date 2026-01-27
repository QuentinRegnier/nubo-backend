CREATE OR REPLACE PROCEDURE content.proc_update_media_owner(
    p_media_id BIGINT,
    p_new_owner_id BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE content.media
    SET 
        owner_id = p_new_owner_id,
        visibility = TRUE -- On s'assure qu'elle devient visible
    WHERE id = p_media_id;
END;
$$;