CREATE OR REPLACE PROCEDURE proc_delete_relation(
    p_user_id BIGINT,
    p_target_id BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM auth.relations
    WHERE (follower_id = p_user_id AND followed_id = p_target_id)
       OR (follower_id = p_target_id AND followed_id = p_user_id);
END;
$$;