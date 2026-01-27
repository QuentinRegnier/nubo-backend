CREATE OR REPLACE PROCEDURE proc_update_user_role_conversation(
    p_conversation_id BIGINT,
    p_user_id BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Met à jour la table 'members'
    UPDATE members
    SET 
        -- Bascule le rôle entre 0 et 1
        -- Si le rôle est 0 (membre), il devient 1 (admin)
        -- Si le rôle est 1 (admin), il devient 0 (membre)
        -- Si le rôle est 2 (créateur) ou autre, il reste inchangé
        role = CASE
                 WHEN role = 0 THEN 1
                 WHEN role = 1 THEN 0
                 ELSE role 
               END
    WHERE 
        conversation_id = p_conversation_id 
        AND user_id = p_user_id;
END;
$$;