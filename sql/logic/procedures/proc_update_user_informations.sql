CREATE OR REPLACE PROCEDURE proc_update_user_informations(
    p_user_id UUID,
    p_email TEXT,
    p_phone TEXT,
    p_password_hash TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE users
    SET email = COALESCE(p_email, email),
        phone = COALESCE(p_phone, phone),
        password_hash = COALESCE(p_password_hash, password_hash),
        updated_at = now()
    WHERE id = p_user_id;
END;
$$;
