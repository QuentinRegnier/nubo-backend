CREATE OR REPLACE FUNCTION content.func_load_media(
    p_owner_id UUID DEFAULT NULL,           -- filtrer sur un utilisateur spécifique (NULL = tous)
    p_media_ids UUID[] DEFAULT NULL,        -- liste d'IDs de médias à charger (NULL = aucun filtre)
    p_order_mode SMALLINT DEFAULT 0         -- 0=plus récents, 1=plus anciens
)
RETURNS TABLE(
    id UUID,
    owner_id UUID,
    storage_path TEXT,
    visibility BOOLEAN,
    created_at TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        m.id,
        m.owner_id,
        m.storage_path,
        m.visibility,
        m.created_at
    FROM content.media m
    WHERE
        m.visibility = TRUE
        AND (p_owner_id IS NULL OR m.owner_id = p_owner_id)
        AND (p_media_ids IS NULL OR m.id = ANY(p_media_ids))
    ORDER BY
        CASE WHEN p_order_mode = 0 THEN m.created_at END DESC,  -- plus récents
        CASE WHEN p_order_mode = 1 THEN m.created_at END ASC;   -- plus anciens
END;
$$;