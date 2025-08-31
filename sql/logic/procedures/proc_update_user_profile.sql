CREATE OR REPLACE PROCEDURE proc_update_user_profile(
    p_user_id UUID,
    p_bio TEXT,
    p_profile_picture_id UUID,
    p_location TEXT,
    p_school TEXT,
    p_work TEXT,
    p_badges TEXT[]
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE users
    SET bio = p_bio,
        profile_picture_id = p_profile_picture_id,
        location = p_location,
        school = p_school,
        work = p_work,
        badges = p_badges,
        updated_at = now()
    WHERE id = p_user_id;
END;
$$;
