CREATE SCHEMA auth;
CREATE SCHEMA content;
CREATE SCHEMA messaging;
CREATE SCHEMA moderation;
CREATE SCHEMA views;

------------------------------------------------------------------------------

SET search_path TO auth;
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY, -- id unique de l'utilisateur
    username TEXT UNIQUE NOT NULL, -- nom d'utilisateur unique
    email TEXT UNIQUE NOT NULL, -- email unique
    email_verified BOOLEAN DEFAULT FALSE, -- email vérifié
    phone TEXT UNIQUE, -- numéro de téléphone unique
    phone_verified BOOLEAN DEFAULT FALSE, -- numéro de téléphone vérifié
    password_hash TEXT NOT NULL, -- mot de passe haché
    first_name TEXT NOT NULL, -- prénom
    last_name TEXT NOT NULL, -- nom de famille
    birthdate DATE, -- date de naissance
    sex SMALLINT, -- sexe
    bio TEXT, -- biographie
    profile_picture_id BIGINT, -- id de l'image de profil
    grade SMALLINT NOT NULL DEFAULT 0, -- grade de l'utilisateur
    location TEXT, -- localisation de l'utilisateur
    school TEXT, -- école
    work TEXT, -- emplois
    badges TEXT[], -- badges
    desactivated BOOLEAN DEFAULT FALSE, -- compte désactivé
    banned BOOLEAN DEFAULT FALSE, -- compte banni
    ban_reason TEXT DEFAULT NULL, -- raison du bannissement
    ban_expires_at TIMESTAMPTZ DEFAULT NULL, -- date d'expiration du bannissement
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    updated_at TIMESTAMPTZ DEFAULT now() -- date de mise à jour
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE TABLE IF NOT EXISTS relations (
    id BIGSERIAL PRIMARY KEY, -- id unique du suivi
    primary_id BIGINT REFERENCES users(id), -- id de l'utilisateur qui suit
    secondary_id BIGINT REFERENCES users(id), -- id de l'utilisateur suivi
    state SMALLINT DEFAULT 1, -- état du suivi (2 = amis, 1 = suivi, 0 = inactif, -1 = bloqué)
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    UNIQUE(secondary_id, primary_id)
);

CREATE INDEX idx_relations_primary_id ON relations(primary_id);
CREATE INDEX idx_relations_secondary_id ON relations(secondary_id);
CREATE TABLE IF NOT EXISTS sessions (
    id BIGSERIAL PRIMARY KEY,        -- id unique de la session
    user_id BIGINT REFERENCES users(id) NOT NULL,          -- id de l'utilisateur
    refresh_token TEXT NOT NULL,                          -- token de rafraîchissement
    device_token JSONB,                           -- identifiant unique de l'appareil
    device_info JSONB,                                    -- informations sur l'appareil (OS, modèle, version...)
    ip_history INET[],                                    -- historique des IP utilisées
    created_at TIMESTAMPTZ DEFAULT now(),                -- date de création
    expires_at TIMESTAMPTZ                                -- date d'expiration
);
CREATE UNIQUE INDEX idx_sessions_user_device 
  ON sessions(user_id, device_token);
CREATE TABLE IF NOT EXISTS user_settings (
    id BIGSERIAL PRIMARY KEY, -- id unique des paramètres utilisateur
    user_id BIGINT UNIQUE REFERENCES users(id) ON DELETE CASCADE, -- id unique de l'utilisateur
    privacy JSONB, -- paramètres de confidentialité
    notifications JSONB, -- paramètres de notification
    language TEXT, -- langue
    theme SMALLINT NOT NULL DEFAULT 0 -- thème clair/sombre
);
CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);
CREATE OR REPLACE FUNCTION auth.func_create_relation(
    p_primary_id BIGINT,
    p_secondary_id BIGINT,
    p_state SMALLINT DEFAULT 1
) RETURNS BIGINT AS $$
DECLARE
    v_relation_id BIGINT;
BEGIN
    INSERT INTO auth.relations(primary_id, secondary_id, state)
    VALUES (p_primary_id, p_secondary_id, p_state)
    ON CONFLICT (primary_id, secondary_id) DO UPDATE
    SET state = excluded.state
    RETURNING id INTO v_relation_id;

    RETURN v_relation_id;
END;
$$ LANGUAGE plpgsql;
CREATE OR REPLACE FUNCTION auth.func_create_session(
    p_user_id BIGINT,
    p_refresh_token TEXT,
    p_device_token TEXT,
    p_device_info JSONB,
    p_ip_history INET[],
    p_expires_at TIMESTAMPTZ
) RETURNS BIGINT AS $$
DECLARE
    v_session_id BIGINT;
BEGIN
    INSERT INTO auth.sessions (
        user_id, refresh_token, device_token, device_info, ip_history, expires_at
    ) VALUES (
        p_user_id, p_refresh_token, p_device_token, p_device_info, p_ip_history, p_expires_at
    )
    RETURNING id INTO v_session_id;

    RETURN v_session_id;
END;
$$ LANGUAGE plpgsql;
CREATE OR REPLACE FUNCTION auth.func_create_user_settings(
    p_user_id BIGINT,
    p_privacy JSONB,
    p_notifications JSONB,
    p_language TEXT,
    p_theme SMALLINT DEFAULT 0
) RETURNS BIGINT AS $$
DECLARE
    v_settings_id BIGINT;
BEGIN
    INSERT INTO auth.user_settings (
        user_id, privacy, notifications, language, theme
    ) VALUES (
        p_user_id, p_privacy, p_notifications, p_language, p_theme
    )
    RETURNING id INTO v_settings_id;

    RETURN v_settings_id;
END;
$$ LANGUAGE plpgsql;
CREATE OR REPLACE FUNCTION auth.func_create_user(
    p_username TEXT,
    p_email TEXT,
    p_phone TEXT,
    p_password_hash TEXT,
    p_first_name TEXT,
    p_last_name TEXT,
    p_birthdate DATE,
    p_sex SMALLINT,
    p_bio TEXT,
    p_profile_picture_id BIGINT,
    p_grade SMALLINT,
    p_location TEXT,
    p_school TEXT,
    p_work TEXT,
    p_badges TEXT[],
    p_desactivated BOOLEAN,
    p_banned BOOLEAN,
    p_ban_reason TEXT,
    p_ban_expires_at TIMESTAMPTZ,
    p_refresh_token TEXT,
    p_device_info JSONB,
    p_device_token TEXT, 
    p_ip_history INET[],
    p_expires_at TIMESTAMPTZ,
    OUT v_user_id BIGINT,
    OUT v_created_at_user TIMESTAMPTZ,
    OUT v_updated_at_user TIMESTAMPTZ,
    OUT v_session_id BIGINT,
    OUT v_created_at_session TIMESTAMPTZ
) LANGUAGE plpgsql 
AS $$
BEGIN
    -- 1️⃣ Créer l'utilisateur
    INSERT INTO auth.users (
        username, email, phone, password_hash, first_name, last_name,
        birthdate, sex, bio, profile_picture_id, grade, location, school, work, badges,
        desactivated, banned, ban_reason, ban_expires_at
    ) VALUES (
        p_username, p_email, p_phone, p_password_hash, p_first_name, p_last_name,
        p_birthdate, p_sex, p_bio, p_profile_picture_id, p_grade, p_location, p_school, p_work, p_badges,
        p_desactivated, p_banned, p_ban_reason, p_ban_expires_at
    )
    RETURNING id, created_at, updated_at INTO v_user_id, v_created_at_user, v_updated_at_user;

    -- 2️⃣ Créer la session (user_settings supprimé)
    INSERT INTO auth.sessions (user_id, refresh_token, device_info, device_token, ip_history, expires_at)
    VALUES (v_user_id, p_refresh_token, p_device_info, p_device_token::jsonb, p_ip_history, p_expires_at)
    RETURNING id, created_at INTO v_session_id, v_created_at_session;
END;
$$;
CREATE OR REPLACE FUNCTION auth.func_load_relations(
    p_id BIGINT DEFAULT NULL,
    p_primary_id BIGINT DEFAULT NULL,
    p_secondary_id BIGINT DEFAULT NULL,
    p_state SMALLINT DEFAULT NULL
)
RETURNS TABLE (
    id BIGINT,
    primary_id BIGINT,
    secondary_id BIGINT,
    state SMALLINT,
    created_at TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        r.id,
        r.primary_id,
        r.secondary_id,
        r.state,
        r.created_at
    FROM auth.relations AS r
    WHERE
        (p_id IS NULL OR r.id = p_id)
        AND (p_primary_id IS NULL OR r.primary_id = p_primary_id)
        AND (p_secondary_id IS NULL OR r.secondary_id = p_secondary_id)
        AND (p_state IS NULL OR r.state = p_state);
END;
$$ LANGUAGE plpgsql STABLE;
CREATE OR REPLACE FUNCTION auth.func_load_sessions(
    p_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL,
    p_device_token TEXT DEFAULT NULL,
    p_refresh_token TEXT DEFAULT NULL
)
RETURNS TABLE (
    id BIGINT,
    user_id BIGINT,
    refresh_token TEXT,
    device_token TEXT,
    device_info JSONB,
    ip_history INET[],
    created_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ
) AS $$
BEGIN
    -- If a device token is provided, return only the corresponding session
    IF p_device_token IS NOT NULL THEN
        RETURN QUERY
        SELECT
            s.id,
            s.user_id,
            s.refresh_token,
            s.device_token,
            s.device_info,
            s.ip_history,
            s.created_at,
            s.expires_at
        FROM auth.sessions AS s
        WHERE s.device_token = p_device_token
        LIMIT 1;
        RETURN;
    END IF;

    RETURN QUERY
    SELECT
        s.id,
        s.user_id,
        s.refresh_token,
        s.device_token,
        s.device_info,
        s.ip_history,
        s.created_at,
        s.expires_at
    FROM auth.sessions AS s
    WHERE
        (p_id IS NULL OR s.id = p_id)
        AND (p_user_id IS NULL OR s.user_id = p_user_id)
        AND (p_device_token IS NULL OR s.device_token = p_device_token)
        AND (p_refresh_token IS NULL OR s.refresh_token = p_refresh_token);
END;
$$ LANGUAGE plpgsql STABLE;
CREATE OR REPLACE FUNCTION auth.func_load_user_settings(
    p_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
RETURNS TABLE (
    id BIGINT,
    user_id BIGINT,
    privacy JSONB,
    notifications JSONB,
    language TEXT,
    theme SMALLINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        s.id,
        s.user_id,
        s.privacy,
        s.notifications,
        s.language,
        s.theme
    FROM auth.user_settings AS s
    WHERE
        (p_id IS NULL OR s.id = p_id)
        AND (p_user_id IS NULL OR s.user_id = p_user_id);
END;
$$ LANGUAGE plpgsql STABLE;
CREATE OR REPLACE FUNCTION auth.func_load_user(
    p_id BIGINT DEFAULT NULL,
    p_username TEXT DEFAULT NULL,
    p_email TEXT DEFAULT NULL,
    p_phone TEXT DEFAULT NULL
)
RETURNS TABLE (
    id BIGINT,
    username TEXT,
    email TEXT,
    email_verified BOOLEAN,
    phone TEXT,
    phone_verified BOOLEAN,
    first_name TEXT,
    last_name TEXT,
    birthdate DATE,
    sex SMALLINT,
    bio TEXT,
    profile_picture_id BIGINT,
    grade SMALLINT,
    location TEXT,
    school TEXT,
    work TEXT,
    badges TEXT[],
    desactivated BOOLEAN,
    banned BOOLEAN,
    ban_reason TEXT,
    ban_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        u.id,
        u.username,
        u.email,
        u.email_verified,
        u.phone,
        u.phone_verified,
        u.first_name,
        u.last_name,
        u.birthdate,
        u.sex,
        u.bio,
        u.profile_picture_id,
        u.grade,
        u.location,
        u.school,
        u.work,
        u.badges,
        u.desactivated,
        u.banned,
        u.ban_reason,
        u.ban_expires_at,
        u.created_at,
        u.updated_at
    FROM auth.users AS u
    WHERE
        (p_id IS NULL OR u.id = p_id)
        AND (p_username IS NULL OR u.username = p_username)
        AND (p_email IS NULL OR u.email = p_email)
        AND (p_phone IS NULL OR u.phone = p_phone);
END;
$$ LANGUAGE plpgsql STABLE;
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
CREATE OR REPLACE PROCEDURE proc_delete_session(
    p_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL,
    p_device_token TEXT DEFAULT NULL,
    p_refresh_token TEXT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_user_id IS NULL AND p_device_token IS NULL AND p_refresh_token IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to delete a session';
    END IF;

    DELETE FROM sessions
    WHERE (id = p_id OR p_id IS NULL)
      AND (user_id = p_user_id OR p_user_id IS NULL)
      AND (device_token = p_device_token OR p_device_token IS NULL)
      AND (refresh_token = p_refresh_token OR p_refresh_token IS NULL);
END;
$$;
CREATE OR REPLACE PROCEDURE proc_delete_user_settings(
    p_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_user_id IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to delete a user setting';
    END IF;

    DELETE FROM user_settings
    WHERE (id = p_id OR p_id IS NULL)
      AND (user_id = p_user_id OR p_user_id IS NULL);
END;
$$;
CREATE OR REPLACE PROCEDURE proc_delete_user(
    p_id BIGINT DEFAULT NULL,
    p_username TEXT DEFAULT NULL,
    p_email TEXT DEFAULT NULL,
    p_phone TEXT DEFAULT NULL,
    p_ban BOOLEAN DEFAULT FALSE,
    p_ban_reason TEXT DEFAULT NULL,
    p_ban_expires_at TIMESTAMPTZ DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_username IS NULL AND p_email IS NULL AND p_phone IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to deactivate or ban a user';
    END IF;

    UPDATE users
    SET desactivated = TRUE,
        banned = p_ban,
        ban_reason = CASE WHEN p_ban THEN p_ban_reason ELSE NULL END,
        ban_expires_at = CASE WHEN p_ban THEN p_ban_expires_at ELSE NULL END,
        updated_at = now()
    WHERE (id = p_id OR p_id IS NULL)
      AND (username = p_username OR p_username IS NULL)
      AND (email = p_email OR p_email IS NULL)
      AND (phone = p_phone OR p_phone IS NULL);
END;
$$;
CREATE OR REPLACE PROCEDURE proc_update_relation(
    p_primary_id BIGINT,
    p_secondary_id BIGINT,
    p_state SMALLINT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE auth.relations
    SET state = COALESCE(p_state, state)
    WHERE primary_id = p_primary_id 
      AND secondary_id = p_secondary_id;
END;
$$;
CREATE OR REPLACE PROCEDURE proc_update_sessions(
    p_id BIGINT,
    p_refresh_token TEXT DEFAULT NULL,
    p_device_info JSONB DEFAULT NULL,
    p_ip_history INET[] DEFAULT NULL,
    p_expires_at TIMESTAMPTZ DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE sessions
    SET
        refresh_token = COALESCE(p_refresh_token, refresh_token),
        device_info   = COALESCE(p_device_info, device_info),
        ip_history    = COALESCE(p_ip_history, ip_history),
        expires_at    = COALESCE(p_expires_at, expires_at)
    WHERE id = p_id;
END;
$$;
CREATE OR REPLACE PROCEDURE proc_update_user_settings(
    p_id BIGINT,
    p_user_id BIGINT,
    p_privacy JSONB DEFAULT NULL,
    p_notifications JSONB DEFAULT NULL,
    p_language TEXT DEFAULT NULL,
    p_theme TEXT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE user_settings
    SET 
        privacy           = COALESCE(p_privacy, privacy),
        notifications     = COALESCE(p_notifications, notifications),
        language          = COALESCE(p_language, language),
        theme             = COALESCE(p_theme, theme)
    WHERE (id = p_id OR p_id IS NULL)
      AND (user_id = p_user_id OR p_user_id IS NULL);
END;
$$;
CREATE OR REPLACE PROCEDURE proc_update_user(
    p_user_id BIGINT,
    p_username TEXT,
    p_email TEXT,
    p_email_verified BOOLEAN,
    p_phone TEXT,
    p_phone_verified BOOLEAN,
    p_password_hash TEXT,
    p_first_name TEXT DEFAULT NULL,
    p_last_name TEXT DEFAULT NULL,
    p_profile_picture_id BIGINT DEFAULT NULL,
    p_location TEXT DEFAULT NULL,
    p_school TEXT DEFAULT NULL,
    p_work TEXT DEFAULT NULL,
    p_desactivated BOOLEAN DEFAULT NULL,
    p_updated_at TIMESTAMPTZ DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE users
    SET username = COALESCE(p_username, username),
        email = COALESCE(p_email, email),
        phone = COALESCE(p_phone, phone),
        password_hash = COALESCE(p_password_hash, password_hash),
        first_name = COALESCE(p_first_name, first_name),
        last_name = COALESCE(p_last_name, last_name),
        profile_picture_id = COALESCE(p_profile_picture_id, profile_picture_id),
        location = COALESCE(p_location, location),
        school = COALESCE(p_school, school),
        work = COALESCE(p_work, work),
        updated_at = COALESCE(p_updated_at, now())
    WHERE id = p_user_id;
END;
$$;
------------------------------------------------------------------------------

SET search_path TO content;
CREATE TABLE IF NOT EXISTS posts (
    id BIGSERIAL PRIMARY KEY, -- id unique du post
    user_id BIGINT REFERENCES auth.users(id) NOT NULL, -- id de l'utilisateur
    content TEXT, -- contenu du post
    media_ids BIGINT[], -- ids des médias associés
    visibility SMALLINT DEFAULT 0, -- visibilité (2= supprimer, 1 = amis, 0 = public)
    location TEXT, -- localisation
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    updated_at TIMESTAMPTZ DEFAULT now() -- date de mise à jour
);
CREATE TABLE IF NOT EXISTS comments (
    id BIGSERIAL PRIMARY KEY, -- id unique du commentaire
    post_id BIGINT REFERENCES posts(id) ON DELETE CASCADE, -- id du post
    user_id BIGINT REFERENCES auth.users(id), -- id de l'utilisateur
    content TEXT, -- contenu du commentaire
    visibility BOOLEAN DEFAULT TRUE, -- visibilité du commentaire
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);
CREATE TABLE IF NOT EXISTS likes (
    id BIGSERIAL PRIMARY KEY, -- id unique du like
    target_type SMALLINT NOT NULL, -- type de la cible (0 = post, 1 = message, 2 = commentaire)
    target_id BIGINT NOT NULL, -- id de la cible
    user_id BIGINT REFERENCES auth.users(id), -- id de l'utilisateur
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    UNIQUE(target_type, target_id, user_id)
);
CREATE TABLE IF NOT EXISTS media (
    id BIGSERIAL PRIMARY KEY, -- id unique du média
    owner_id BIGINT REFERENCES auth.users(id), -- id du propriétaire
    storage_path TEXT, -- chemin de stockage
    visibility BOOLEAN DEFAULT TRUE, -- true si le media est utilisé dans un post/un message/une image de profil publique
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);
CREATE INDEX idx_posts_user_created ON posts(user_id, created_at DESC);
CREATE INDEX idx_media_owner ON media(owner_id);
CREATE INDEX idx_media_created ON media(created_at);
CREATE INDEX idx_likes_target ON likes(target_type, target_id);
CREATE INDEX idx_comments_post_created ON comments(post_id, created_at DESC);
CREATE OR REPLACE FUNCTION content.func_add_like(
    p_target_type SMALLINT,
    p_target_id BIGINT,
    p_user_id BIGINT
) RETURNS BIGINT AS $$
DECLARE
    v_like_id BIGINT;
BEGIN
    INSERT INTO content.likes(target_type, target_id, user_id)
    VALUES (p_target_type, p_target_id, p_user_id)
    ON CONFLICT (target_type, target_id, user_id) DO NOTHING
    RETURNING id INTO v_like_id;

    -- Si le like existait déjà, on récupère son id
    IF v_like_id IS NULL THEN
        SELECT id INTO v_like_id
        FROM content.likes
        WHERE target_type = p_target_type
          AND target_id = p_target_id
          AND user_id = p_user_id;
    END IF;

    RETURN v_like_id;
END;
$$ LANGUAGE plpgsql;
CREATE OR REPLACE FUNCTION content.func_create_comment(
    p_post_id BIGINT,
    p_user_id BIGINT,
    p_content TEXT
) RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
    v_comment_id BIGINT;
BEGIN
    INSERT INTO content.comments(post_id, user_id, content)
    VALUES (p_post_id, p_user_id, p_content)
    RETURNING id INTO v_comment_id;

    RETURN v_comment_id;
END;
$$;
CREATE OR REPLACE FUNCTION content.func_create_media(
    p_owner_id BIGINT,
    p_storage_path TEXT
) RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
    v_media_id BIGINT;
BEGIN
    INSERT INTO content.media(owner_id, storage_path)
    VALUES (p_owner_id, p_storage_path)
    RETURNING id INTO v_media_id;

    RETURN v_media_id;
END;
$$;
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
CREATE OR REPLACE FUNCTION content.func_load_comments(
    p_post_id BIGINT DEFAULT NULL,          -- filtrer sur un post spécifique (NULL = tous les posts)
    p_user_id BIGINT DEFAULT NULL,          -- filtrer sur un utilisateur spécifique (NULL = tous)
    p_limit INT DEFAULT 100,              -- limite de résultats
    p_order_mode SMALLINT DEFAULT 0       -- 0=plus récents, 1=plus likés, 2=plus anciens
)
RETURNS TABLE(
    id BIGINT,
    content TEXT,
    user_id BIGINT,
    post_id BIGINT,
    created_at TIMESTAMPTZ,
    like_count INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        c.id,
        c.content,
        c.user_id,
        c.post_id,
        c.created_at,
        COALESCE(
            (SELECT COUNT(*) 
             FROM content.likes l 
             WHERE l.target_type = 2 
               AND l.target_id = c.id),
        0) AS like_count
    FROM content.comments c
    WHERE
        c.visibility = TRUE  -- ✅ on ignore les commentaires invisibles
        AND (p_post_id IS NULL OR c.post_id = p_post_id)
        AND (p_user_id IS NULL OR c.user_id = p_user_id)
    ORDER BY
        CASE WHEN p_order_mode = 0 THEN c.created_at END DESC,   -- plus récents
        CASE WHEN p_order_mode = 1 THEN like_count END DESC,      -- plus likés
        CASE WHEN p_order_mode = 2 THEN c.created_at END ASC      -- plus anciens
    LIMIT p_limit;
END;
$$;
CREATE OR REPLACE FUNCTION content.func_load_likes(
    p_target_type SMALLINT DEFAULT NULL,  -- 0=post, 1=message, 2=commentaire, NULL = tous
    p_target_id BIGINT DEFAULT NULL,        -- id de la cible (si ciblé)
    p_user_id BIGINT DEFAULT NULL,          -- id utilisateur (si on veut les likes d'un user)
    p_limit INT DEFAULT 100,              -- limite de résultats
    p_order_mode SMALLINT DEFAULT 0       -- 0=plus récents, 1=plus anciens
)
RETURNS TABLE(
    id BIGINT,
    target_type SMALLINT,
    target_id BIGINT,
    user_id BIGINT,
    created_at TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT
        l.id,
        l.target_type,
        l.target_id,
        l.user_id,
        l.created_at
    FROM content.likes l
    WHERE
        (p_target_type IS NULL OR l.target_type = p_target_type)
        AND (p_target_id IS NULL OR l.target_id = p_target_id)
        AND (p_user_id IS NULL OR l.user_id = p_user_id)
    ORDER BY
        CASE WHEN p_order_mode = 0 THEN l.created_at END DESC,
        CASE WHEN p_order_mode = 1 THEN l.created_at END ASC
    LIMIT p_limit;
END;
$$;
CREATE OR REPLACE FUNCTION content.func_load_media(
    p_owner_id BIGINT DEFAULT NULL,           -- filtrer sur un utilisateur spécifique (NULL = tous)
    p_media_ids BIGINT[] DEFAULT NULL,        -- liste d'IDs de médias à charger (NULL = aucun filtre)
    p_order_mode SMALLINT DEFAULT 0         -- 0=plus récents, 1=plus anciens
)
RETURNS TABLE(
    id BIGINT,
    owner_id BIGINT,
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
CREATE OR REPLACE FUNCTION content.func_load_posts(
    p_user_id BIGINT DEFAULT NULL,            -- filtrer sur un utilisateur spécifique (NULL = tous)
    p_post_ids BIGINT[] DEFAULT NULL,         -- liste d'IDs de posts à charger (NULL = aucun filtre)
    p_visibility SMALLINT[] DEFAULT ARRAY[0,1], -- visibilités autorisées (0=public,1=amis)
    p_order_mode SMALLINT DEFAULT 0         -- 0=plus récents, 1=plus anciens
)
RETURNS TABLE(
    id BIGINT,
    user_id BIGINT,
    content TEXT,
    media_ids BIGINT[],
    visibility SMALLINT,
    location TEXT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    like_count INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        p.id,
        p.user_id,
        p.content,
        p.media_ids,
        p.visibility,
        p.location,
        p.created_at,
        p.updated_at,
        COALESCE(
            (SELECT COUNT(*) 
             FROM content.likes l 
             WHERE l.target_type = 0 
               AND l.target_id = p.id),
        0) AS like_count
    FROM content.posts p
    WHERE
        p.visibility != 2  -- ne pas charger les posts supprimés
        AND (p_user_id IS NULL OR p.user_id = p_user_id)
        AND (p_post_ids IS NULL OR p.id = ANY(p_post_ids))
        AND (p.visibility = ANY(p_visibility))
    ORDER BY
        CASE WHEN p_order_mode = 0 THEN p.created_at END DESC,  -- plus récents
        CASE WHEN p_order_mode = 1 THEN p.created_at END ASC;   -- plus anciens
END;
$$;
CREATE OR REPLACE PROCEDURE proc_delete_comment(
    p_id BIGINT DEFAULT NULL,
    p_post_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_post_id IS NULL AND p_user_id IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to hide a comment';
    END IF;

    UPDATE comments
    SET visibility = FALSE
    WHERE (id = p_id OR p_id IS NULL)
      AND (post_id = p_post_id OR p_post_id IS NULL)
      AND (user_id = p_user_id OR p_user_id IS NULL);
END;
$$;
CREATE OR REPLACE PROCEDURE proc_delete_media(
    p_id BIGINT DEFAULT NULL,
    p_owner_id BIGINT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_owner_id IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to disable a media';
    END IF;

    UPDATE media
    SET visibility = FALSE
    WHERE (id = p_id OR p_id IS NULL)
      AND (owner_id = p_owner_id OR p_owner_id IS NULL);
END;
$$;
CREATE OR REPLACE PROCEDURE proc_delete_post(
    p_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Vérification qu'au moins un critère est fourni
    IF p_id IS NULL AND p_user_id IS NULL THEN
        RAISE EXCEPTION 'At least one criterion must be provided to hide a post';
    END IF;

    UPDATE posts
    SET visibility = 2,
        updated_at = now()
    WHERE (id = p_id OR p_id IS NULL)
      AND (user_id = p_user_id OR p_user_id IS NULL);
END;
$$;
CREATE OR REPLACE PROCEDURE proc_remove_like(
    p_target_type SMALLINT,
    p_target_id BIGINT,
    p_user_id BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM content.likes
    WHERE target_type = p_target_type
      AND target_id = p_target_id
      AND user_id = p_user_id;
END;
$$;
CREATE OR REPLACE PROCEDURE proc_update_comment(
    p_comment_id BIGINT,
    p_user_id BIGINT,
    p_content TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE content.comments
    SET content = p_content
    WHERE id = p_comment_id AND user_id = p_user_id;
END;
$$;
CREATE OR REPLACE PROCEDURE proc_update_post(
    p_post_id BIGINT,
    p_user_id BIGINT,
    p_content TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE content.posts
    SET content = p_content,
        updated_at = now()
    WHERE id = p_post_id AND user_id = p_user_id;
END;
$$;
CREATE OR REPLACE PROCEDURE content.proc_update_media_owner(
    p_media_id BIGINT,
    p_new_owner_id BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE content.media
    SET 
        owner_id = p_new_owner_id,
        visibility = TRUE -- On s'assure qu'elle devient visible
    WHERE id = p_media_id;
END;
$$;
------------------------------------------------------------------------------

SET search_path TO messaging;
CREATE TABLE IF NOT EXISTS conversations (
    id BIGSERIAL PRIMARY KEY, -- id unique de la conversation
    type SMALLINT, -- type de la conversation (0 = message privée, 1 = groupe, 2 = communauté, 3 = annonce)
    title TEXT DEFAULT NULL, -- titre de la conversation
    last_message_id BIGINT UNIQUE DEFAULT NULL, -- id du dernier message
    last_read_by_all_message_id BIGINT DEFAULT NULL, -- id du dernier message lu par tous
    state SMALLINT DEFAULT 0, -- état de la conversation (0 = active, 1 = supprimée, 2 = archivée)
    laws SMALLINT[], -- lois applicables à la conversation
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_conversations_last_message ON conversations(last_message_id);
CREATE TABLE IF NOT EXISTS members (
    id BIGSERIAL PRIMARY KEY, -- id unique du membre
    conversation_id BIGINT REFERENCES conversations(id), -- id de la conversation
    user_id BIGINT REFERENCES auth.users(id), -- id de l'utilisateur
    role SMALLINT DEFAULT 0, -- rôle du membre (0 = membre, 1 = admin, 2 = créateur)
    joined_at TIMESTAMPTZ DEFAULT now(), -- date d'adhésion
    unread_count INT DEFAULT 0, -- nombre de messages non lus
    UNIQUE(conversation_id, user_id)
);
CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY, -- id unique du message
    conversation_id BIGINT REFERENCES conversations(id), -- id de la conversation
    sender_id BIGINT NOT NULL, -- id de l'expéditeur
    message_type SMALLINT NOT NULL DEFAULT 0, -- 0=text, 1=image, 2=publication, 3=vocal, 4=vidéo
    visibility BOOLEAN DEFAULT TRUE, -- true si le message est visible par les membres (ou supprimé par l'expéditeur)
    content TEXT, -- contenu du message
    attachments JSONB, -- pointeurs vers fichiers S3 / metadata
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_message_conv_created ON messages(conversation_id, created_at DESC);
CREATE OR REPLACE FUNCTION messaging.func_create_conversation(
    p_type SMALLINT,
    p_members JSONB, -- tableau JSON du type [{ "user_id": "...", "role": 2}, ...]
    p_title TEXT DEFAULT NULL,
    p_laws SMALLINT[] DEFAULT '{}'
) RETURNS BIGINT AS $$
DECLARE
    v_conversation_id BIGINT;
    v_user_ids BIGINT[];
    v_roles SMALLINT[];
BEGIN
    -- 1️⃣ Création de la conversation
    INSERT INTO messaging.conversations (type, title, laws)
    VALUES (p_type, p_title, p_laws)
    RETURNING id INTO v_conversation_id;

    -- 2️⃣ Extraction des données membres depuis le JSON fourni
    SELECT
        array_agg((m->>'user_id')::BIGINT),
        array_agg(COALESCE((m->>'role')::SMALLINT, 0))
    INTO v_user_ids, v_roles
    FROM jsonb_array_elements(p_members) AS m;

    -- 3️⃣ Appel à la fonction d’ajout multiple de membres
    PERFORM messaging.func_create_members_bulk(
        v_conversation_id,
        v_user_ids,
        v_roles
    );

    -- 4️⃣ Retourne l’ID de la conversation nouvellement créée
    RETURN v_conversation_id;
END;
$$ LANGUAGE plpgsql;
CREATE OR REPLACE FUNCTION messaging.func_create_members(
    p_conversation_id BIGINT,
    p_user_ids BIGINT[],
    p_roles SMALLINT[] DEFAULT '{}'
)
RETURNS BIGINT[]
LANGUAGE sql
AS $$
WITH input_data AS (
    SELECT
        unnest(p_user_ids) AS user_id,
        unnest(
            CASE 
                WHEN array_length(p_roles,1) IS NULL THEN array_fill(0::smallint, ARRAY[array_length(p_user_ids,1)])
                ELSE p_roles
            END
        ) AS role
),
inserted AS (
    INSERT INTO messaging.members (conversation_id, user_id, role)
    SELECT p_conversation_id, user_id, role
    FROM input_data
    ON CONFLICT (conversation_id, user_id) DO UPDATE
        SET role = EXCLUDED.role
    RETURNING id
)
SELECT array_agg(id) FROM inserted;
$$;
CREATE OR REPLACE FUNCTION messaging.func_create_message(
    p_conversation_id BIGINT,
    p_sender_id BIGINT,
    p_content TEXT,
    p_attachments JSONB,
    p_message_type SMALLINT DEFAULT 0,
    p_visibility BOOLEAN DEFAULT TRUE
) RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
    v_message_id BIGINT;
    v_user_id BIGINT;
BEGIN
    -- 1️⃣ Insertion du message
    INSERT INTO messaging.messages(conversation_id, sender_id, message_type, visibility, content, attachments)
    VALUES (p_conversation_id, p_sender_id, p_message_type, p_visibility, p_content, p_attachments)
    RETURNING id INTO v_message_id;

    -- 2️⃣ Met à jour la conversation via la procédure proc_update_conversation
    CALL proc_update_conversation(
        p_conversation_id := p_conversation_id,
        p_last_message_id := v_message_id
    );

    -- 3️⃣ Met à jour les unread_count pour tous les membres sauf l’expéditeur
    FOR v_user_id IN
        SELECT user_id
        FROM messaging.members
        WHERE conversation_id = p_conversation_id
          AND user_id <> p_sender_id
    LOOP
        CALL proc_update_member(
            p_conversation_id := p_conversation_id,
            p_user_id := v_user_id,
            p_unread_count := (
                SELECT unread_count + 1
                FROM messaging.members
                WHERE conversation_id = p_conversation_id
                  AND user_id = v_user_id
            )
        );
    END LOOP;

    -- 4️⃣ Retourne l’identifiant du message nouvellement créé
    RETURN v_message_id;
END;
$$;
CREATE OR REPLACE FUNCTION messaging.func_load_conversation(
    p_user_id BIGINT
)
-- 1. Définition de la structure de sortie ("liste de structure")
RETURNS TABLE (
    conversation_id BIGINT,
    title TEXT,
    type SMALLINT,
    laws SMALLINT[],
    state SMALLINT,
    created_at TIMESTAMPTZ,
    last_message_id BIGINT,
    last_read_by_all_message_id BIGINT,
    joined_at TIMESTAMPTZ,
    role SMALLINT,
    unread_count INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- 2. Utilisation de RETURN QUERY pour renvoyer l'ensemble des résultats
    RETURN QUERY
    SELECT
        c.id AS conversation_id,
        c.title,
        c.type,
        c.laws,
        c.state,
        c.created_at,
        c.last_message_id,
        c.last_read_by_all_message_id,
        m.joined_at,
        m.role,
        m.unread_count
    FROM
        messaging.members AS m
    -- 3. Jointure pour combiner les tables
    INNER JOIN
        messaging.conversations AS c ON m.conversation_id = c.id
    WHERE
        -- 4. Filtre sur l'utilisateur demandé
        m.user_id = p_user_id
        -- 5. MODIFICATION : Exclut les conversations avec state = 1
        AND c.state <> 1;
END;
$$;
CREATE OR REPLACE FUNCTION messaging.func_load_members(
    p_conversation_id BIGINT
)
-- 1. Définition de la structure de sortie
RETURNS TABLE (
    user_id BIGINT,
    username TEXT,
    grade SMALLINT,
    banned BOOLEAN,
    desactivated BOOLEAN,
    profile_picture_storage_path TEXT,
    role SMALLINT,
    joined_at TIMESTAMPTZ,
    unread_count INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- 2. Utilisation de RETURN QUERY pour renvoyer l'ensemble des résultats
    RETURN QUERY
    SELECT
        u.id AS user_id,
        u.username,
        u.grade,
        u.banned,
        u.desactivated,
        med.storage_path AS profile_picture_storage_path,
        m.role,
        m.joined_at,
        m.unread_count
    FROM
        -- 3. Table de base : les membres de la conversation
        messaging.members AS m
    INNER JOIN
        -- 4. Jointure avec auth.users pour les infos utilisateur
        auth.users AS u ON m.user_id = u.id
    LEFT JOIN
        -- 5. LEFT JOIN avec content.media pour la photo (optionnelle)
        content.media AS med ON u.profile_picture_id = med.id
    WHERE
        -- 6. Filtre sur la conversation demandée
        m.conversation_id = p_conversation_id;
END;
$$;
CREATE OR REPLACE FUNCTION messaging.func_load_messages(
    p_conversation_id BIGINT
) RETURNS SETOF messaging.messages
LANGUAGE sql
AS $$
    SELECT *
    FROM messaging.messages
    WHERE conversation_id = p_conversation_id
      AND visibility = TRUE  -- exclut les messages supprimés
    ORDER BY created_at ASC;
$$;
CREATE OR REPLACE PROCEDURE proc_delete_conversation(
    p_conversation_id BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE messaging.conversations
    SET state = 1
    WHERE id = p_conversation_id;
END;
$$;
CREATE OR REPLACE PROCEDURE proc_delete_member(
    p_conversation_id BIGINT,
    p_user_id BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM messaging.members
    WHERE conversation_id = p_conversation_id AND user_id = p_user_id;
END;
$$;
CREATE OR REPLACE PROCEDURE proc_delete_messages(
    p_message_ids BIGINT[],
    p_user_id BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE messaging.messages
    SET visibility = FALSE
    WHERE id = ANY(p_message_ids)
      AND sender_id = p_user_id
      AND visibility = TRUE;
END;
$$;
CREATE OR REPLACE PROCEDURE proc_update_conversation(
    p_conversation_id BIGINT,
    p_title TEXT DEFAULT NULL,
    p_last_message_id BIGINT DEFAULT NULL,
    p_state SMALLINT DEFAULT NULL,
    p_laws SMALLINT[] DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE conversations
    SET
        title = COALESCE(p_title, title),
        last_message_id = COALESCE(p_last_message_id, last_message_id),
        state = COALESCE(p_state, state),
        laws = COALESCE(p_laws, laws)
    WHERE id = p_conversation_id;
END;
$$;
CREATE OR REPLACE PROCEDURE proc_update_member(
    p_conversation_id BIGINT,
    p_user_id BIGINT,
    p_role SMALLINT DEFAULT NULL,
    p_unread_count INT DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE members
    SET
        role = COALESCE(p_role, role),
        unread_count = COALESCE(p_unread_count, unread_count)
    WHERE conversation_id = p_conversation_id
      AND user_id = p_user_id;
END;
$$;
CREATE OR REPLACE PROCEDURE proc_update_message(
    p_message_id BIGINT,
    p_user_id BIGINT,
    p_content TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE messaging.messages
    SET content = p_content,
        updated_at = now()
    WHERE id = p_message_id
      AND sender_id = p_user_id
      AND message_type = 0 -- uniquement texte
      AND visibility = TRUE;       -- actif
END;
$$;
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
------------------------------------------------------------------------------

SET search_path TO moderation;
CREATE TABLE IF NOT EXISTS reports (
    id BIGSERIAL PRIMARY KEY, -- id unique du rapport
    actor_id BIGINT REFERENCES auth.users(id), -- id de l'utilisateur ayant signalé
    target_type SMALLINT NOT NULL, -- type de la cible (user/post/comment/etc)
    target_id BIGINT NOT NULL, -- id de la cible
    reason TEXT, -- raison du signalement
    rationale TEXT DEFAULT NULL, -- explication des mesures prises
    state SMALLINT DEFAULT 0, -- état du rapport (0=pending, 1=reviewed, 2=resolved)
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_reports_actor ON reports(actor_id);
CREATE INDEX idx_reports_created ON reports(created_at);
CREATE OR REPLACE FUNCTION moderation.func_load_reports_by_state(
    p_state SMALLINT,
    p_limit INT
)
-- 1. Définit la structure de retour (correspond à la table 'reports')
RETURNS TABLE (
    id BIGINT,
    actor_id BIGINT,
    target_type SMALLINT,
    target_id BIGINT,
    reason TEXT,
    rationale TEXT,
    state SMALLINT,
    created_at TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- 2. Retourne le résultat de la requête
    RETURN QUERY
    SELECT
        r.id,
        r.actor_id,
        r.target_type,
        r.target_id,
        r.reason,
        r.rationale,
        r.state,
        r.created_at
    FROM
        moderation.reports AS r
    WHERE
        -- 3. Filtre par l'état demandé
        r.state = p_state
    ORDER BY
        -- 4. Trie par date de création (les "premiers" = les plus anciens)
        r.created_at ASC
    LIMIT
        -- 5. Limite au nombre N demandé
        p_limit;
END;
$$;
CREATE OR REPLACE PROCEDURE proc_create_report(
    p_actor_id BIGINT,
    p_target_type SMALLINT,
    p_target_id BIGINT,
    p_reason TEXT,
    p_state SMALLINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO moderation.reports(actor_id, target_type, target_id, reason, rationale, state)
    VALUES (p_actor_id, p_target_type, p_target_id, p_reason, NULL, p_state);
END;
$$;
CREATE OR REPLACE PROCEDURE moderation.proc_update_report(
    p_report_id BIGINT,
    p_new_state SMALLINT,
    p_new_rationale TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Met à jour le rapport spécifié
    UPDATE moderation.reports
    SET 
        state = p_new_state,
        rationale = p_new_rationale
    WHERE 
        id = p_report_id;
END;
$$;
------------------------------------------------------------------------------

SET search_path TO views;
-- Vue : conversation_summary
-- Résumé d’une conversation par membre :
-- - Dernier message
-- - Nombre de messages non lus
-- - Rôle et date d’adhésion

CREATE OR REPLACE VIEW conversation_summary AS
SELECT
    cm.conversation_id,
    cm.user_id,
    cm.role,
    cm.joined_at,
    cm.unread_count,
    last_msg.id AS last_message_id,
    last_msg.sender_id AS last_sender_id,
    last_msg.message_type AS last_message_type,
    last_msg.content AS last_message_content,
    last_msg.created_at AS last_message_time
FROM messaging.members cm
LEFT JOIN LATERAL (
    SELECT m.*
    FROM messaging.messages m
    WHERE m.conversation_id = cm.conversation_id
    ORDER BY m.created_at DESC
    LIMIT 1
) last_msg ON true;
CREATE OR REPLACE VIEW conversation_participants_view AS
SELECT
    cm.conversation_id,
    cm.user_id,
    u.username,
    u.first_name,
    u.last_name,
    cm.role,
    cm.joined_at,
    cm.unread_count,
    conv.type AS conversation_type,
    conv.title AS conversation_title,
    conv.state AS conversation_state,
    conv.created_at AS conversation_created_at
FROM messaging.members cm
JOIN messaging.conversations conv ON cm.conversation_id = conv.id
JOIN auth.users u ON cm.user_id = u.id;
-- Vue : post_engagement_view
-- Donne le nombre de likes et de commentaires pour chaque post

CREATE OR REPLACE VIEW post_engagement_view AS
SELECT
    p.id AS post_id,
    p.user_id,
    p.content,
    p.media_ids,
    p.visibility,
    p.location,
    p.created_at,
    p.updated_at,
    COALESCE(l.like_count, 0) AS like_count,
    COALESCE(c.comment_count, 0) AS comment_count
FROM content.posts p
LEFT JOIN (
    SELECT target_id, COUNT(*) AS like_count
    FROM content.likes
    WHERE target_type = 0 -- 0 = post
    GROUP BY target_id
) l ON p.id = l.target_id
LEFT JOIN (
    SELECT post_id, COUNT(*) AS comment_count
    FROM content.comments
    GROUP BY post_id
) c ON p.id = c.post_id;
-- Vue : user_public_profile
-- Permet d’exposer uniquement les infos publiques + paramètres utilisateur

CREATE OR REPLACE VIEW user_public_profile AS
SELECT
    u.id AS user_id,
    u.username,
    u.first_name,
    u.last_name,
    u.birthdate,
    u.sex,
    u.bio,
    u.profile_picture_id,
    u.grade,
    u.location,
    u.school,
    u.work,
    u.badges,
    s.privacy,
    s.notifications,
    s.language,
    s.theme
FROM auth.users u
LEFT JOIN auth.user_settings s ON u.id = s.user_id;
CREATE OR REPLACE VIEW user_relations_view AS
SELECT
    r.id AS relation_id,
    r.primary_id AS follower_id,
    u1.username AS follower_username,
    r.secondary_id AS followed_id,
    u2.username AS followed_username,
    r.state,
    r.created_at
FROM auth.relations r
JOIN auth.users u1 ON r.primary_id = u1.id
JOIN auth.users u2 ON r.secondary_id = u2.id;
CREATE OR REPLACE FUNCTION views.func_load_conversation_participants_view(
    p_conversation_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
RETURNS TABLE (
    conversation_id BIGINT,
    user_id BIGINT,
    username TEXT,
    first_name TEXT,
    last_name TEXT,
    role SMALLINT,
    joined_at TIMESTAMPTZ,
    unread_count INT,
    conversation_type SMALLINT,
    conversation_title TEXT,
    conversation_state SMALLINT,
    conversation_created_at TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT *
    FROM views.conversation_participants_view
    WHERE 
        (p_conversation_id IS NULL OR conversation_participants_view.conversation_id = p_conversation_id)
        AND (p_user_id IS NULL OR conversation_participants_view.user_id = p_user_id);
END;
$$;
CREATE OR REPLACE FUNCTION views.func_load_conversation_summary(
    p_conversation_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
RETURNS TABLE (
    conversation_id BIGINT,
    user_id BIGINT,
    role SMALLINT,
    joined_at TIMESTAMPTZ,
    unread_count INT,
    last_message_id BIGINT,
    last_sender_id BIGINT,
    last_message_type SMALLINT,
    last_message_content TEXT,
    last_message_time TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        s.conversation_id,
        s.user_id,
        s.role,
        s.joined_at,
        s.unread_count,
        s.last_message_id,
        s.last_sender_id,
        s.last_message_type,
        s.last_message_content,
        s.last_message_time
    FROM views.conversation_summary AS s -- Correction ici : c'est dans le schéma 'views', pas 'public'
    WHERE 
        (p_conversation_id IS NULL OR s.conversation_id = p_conversation_id)
        AND (p_user_id IS NULL OR s.user_id = p_user_id);
END;
$$;
CREATE OR REPLACE FUNCTION views.func_load_conversation_user_view(
    p_user_id BIGINT DEFAULT NULL
)
RETURNS TABLE (
    user_id BIGINT,
    conversation_ids BIGINT[]
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT *
    FROM views.conversation_user_view
    WHERE 
        (p_user_id IS NULL OR conversation_user_view.user_id = p_user_id);
END;
$$;
CREATE OR REPLACE FUNCTION views.func_load_post_engagement_view(
    p_post_id BIGINT DEFAULT NULL,
    p_user_id BIGINT DEFAULT NULL
)
RETURNS TABLE (
    post_id BIGINT,
    user_id BIGINT,
    content TEXT,
    media_ids BIGINT[],
    visibility SMALLINT,
    location TEXT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    like_count BIGINT,
    comment_count BIGINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT *
    FROM views.post_engagement_view
    WHERE 
        (p_post_id IS NULL OR post_engagement_view.post_id = p_post_id)
        AND (p_user_id IS NULL OR post_engagement_view.user_id = p_user_id);
END;
$$;
CREATE OR REPLACE FUNCTION views.func_load_user_public_profile(
    p_user_id BIGINT DEFAULT NULL
)
RETURNS TABLE (
    user_id BIGINT,
    username TEXT,
    first_name TEXT,
    last_name TEXT,
    birthdate DATE,
    sex SMALLINT,
    bio TEXT,
    profile_picture_id BIGINT,
    grade SMALLINT,
    location TEXT,
    school TEXT,
    work TEXT,
    badges TEXT[],
    privacy JSONB, -- En supposant que 'privacy' est un type JSONB dans user_settings
    notifications JSONB, -- En supposant que 'notifications' est un type JSONB
    language TEXT,
    theme TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Note : J'ai dû deviner les types pour s.privacy, s.notifications, etc.
    -- Ajustez-les (ex: JSONB, TEXT) si nécessaire.
    RETURN QUERY
    SELECT *
    FROM views.user_public_profile
    WHERE 
        (p_user_id IS NULL OR user_public_profile.user_id = p_user_id);
END;
$$;
CREATE OR REPLACE FUNCTION views.func_load_user_relations_view(
    p_follower_id BIGINT DEFAULT NULL,
    p_followed_id BIGINT DEFAULT NULL
)
RETURNS TABLE (
    relation_id BIGINT,
    follower_id BIGINT,
    follower_username TEXT,
    followed_id BIGINT,
    followed_username TEXT,
    state SMALLINT,
    created_at TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT *
    FROM views.user_relations_view
    WHERE 
        (p_follower_id IS NULL OR user_relations_view.follower_id = p_follower_id)
        AND (p_followed_id IS NULL OR user_relations_view.followed_id = p_followed_id);
END;
$$;
CREATE OR REPLACE VIEW conversation_user_view AS
SELECT
    cm.user_id,
    ARRAY_AGG(cm.conversation_id ORDER BY last_msg.created_at DESC) AS conversation_ids
FROM messaging.members cm
JOIN messaging.conversations conv ON conv.id = cm.conversation_id
LEFT JOIN LATERAL (
    SELECT m.id, m.created_at, m.sender_id
    FROM messaging.messages m
    WHERE m.conversation_id = cm.conversation_id
      AND m.sender_id != cm.user_id
    ORDER BY m.created_at DESC
    LIMIT 1
) last_msg ON true
GROUP BY cm.user_id;