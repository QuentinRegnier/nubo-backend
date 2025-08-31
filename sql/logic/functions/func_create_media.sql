CREATE OR REPLACE FUNCTION func_create_media(
    p_owner_id UUID,
    p_storage_path TEXT
) RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_media_id UUID;
BEGIN
    INSERT INTO content.media(owner_id, storage_path)
    VALUES (p_owner_id, p_storage_path)
    RETURNING id INTO v_media_id;

    RETURN v_media_id;
END;
$$;