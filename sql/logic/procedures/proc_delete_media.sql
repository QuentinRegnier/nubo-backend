CREATE OR REPLACE PROCEDURE proc_delete_media(
    p_media_id UUID,
    p_owner_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM content.media
    WHERE id = p_media_id AND owner_id = p_owner_id;
END;
$$;
