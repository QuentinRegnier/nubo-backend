CREATE OR REPLACE FUNCTION views.func_load_conversation_user_view(
    p_user_id UUID DEFAULT NULL
)
RETURNS TABLE (
    user_id UUID,
    conversation_ids UUID[]
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT *
    FROM public.conversation_user_view
    WHERE 
        (p_user_id IS NULL OR conversation_user_view.user_id = p_user_id);
END;
$$;