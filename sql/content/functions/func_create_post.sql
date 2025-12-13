CREATE OR REPLACE FUNCTION content.func_create_post(
    p_user_id BIGINT,
    p_content TEXT,
    p_media_ids BIGINT[],
    p_visibility SMALLINT,
    p_location TEXT
) RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
    v_post_id BIGINT;
BEGIN
    INSERT INTO content.posts(user_id, content, media_ids, visibility, location)
    VALUES (p_user_id, p_content, p_media_ids, p_visibility, p_location)
    RETURNING id INTO v_post_id;

    RETURN v_post_id;
END;
$$;