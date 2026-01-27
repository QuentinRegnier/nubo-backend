CREATE OR REPLACE FUNCTION messaging.func_create_members(
    p_conversation_id BIGINT,
    p_user_ids BIGINT[],
    p_roles SMALLINT[] DEFAULT '{}'
)
RETURNS BIGINT[]  -- on retourne la liste des IDs créés ou mis à jour
LANGUAGE sql
AS $$
WITH input_data AS (
    -- Si p_roles est vide, on remplit avec 0
    SELECT
        unnest(p_user_ids) AS user_id,
        unnest(
            CASE 
                WHEN array_length(p_roles,1) IS NULL THEN array_fill(0::smallint, ARRAY[array_length(p_user_ids,1)])
                ELSE p_roles
            END
        ) AS role
)
INSERT INTO messaging.members (conversation_id, user_id, role)
SELECT p_conversation_id, user_id, role
FROM input_data
ON CONFLICT (conversation_id, user_id) DO UPDATE
    SET role = EXCLUDED.role
RETURNING id;
$$;