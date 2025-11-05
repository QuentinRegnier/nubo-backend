CREATE OR REPLACE PROCEDURE proc_update_user_settings(
    p_user_id UUID,
    p_bio TEXT DEFAULT NULL,
    p_profile_picture_id UUID DEFAULT NULL,
    p_location TEXT DEFAULT NULL,
    p_school TEXT DEFAULT NULL,
    p_work TEXT DEFAULT NULL,
    p_badges TEXT[] DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE users
    SET 
        bio                = COALESCE(p_bio, bio),
        profile_picture_id = COALESCE(p_profile_picture_id, profile_picture_id),
        location           = COALESCE(p_location, location),
        school             = COALESCE(p_school, school),
        work               = COALESCE(p_work, work),
        badges             = COALESCE(p_badges, badges),
        updated_at         = now()
    WHERE id = p_user_id;
END;
$$;