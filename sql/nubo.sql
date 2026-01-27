CREATE SCHEMA auth;
CREATE SCHEMA content;
CREATE SCHEMA messaging;
CREATE SCHEMA moderation;
CREATE SCHEMA views;

------------------------------------------------------------------------------

SET search_path TO auth;
CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur (correspond à int64)
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    email_verified BOOLEAN,             -- Modifié : Plus de valeur par défaut
    phone TEXT UNIQUE,
    phone_verified BOOLEAN,             -- Modifié : Plus de valeur par défaut
    password_hash TEXT NOT NULL,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    birthdate DATE,
    sex SMALLINT,
    bio TEXT,
    profile_picture_id BIGINT,
    grade SMALLINT NOT NULL,            -- Modifié : Plus de valeur par défaut (0)
    location TEXT,
    school TEXT,
    work TEXT,
    badges TEXT[],
    desactivated BOOLEAN,               -- Modifié : Plus de valeur par défaut
    banned BOOLEAN,                     -- Modifié : Plus de valeur par défaut
    ban_reason TEXT,
    ban_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_phone ON users(phone);
CREATE TABLE IF NOT EXISTS relations (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    primary_id BIGINT REFERENCES users(id),
    secondary_id BIGINT REFERENCES users(id),
    state SMALLINT,                     -- Modifié : Plus de valeur par défaut (1)
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    UNIQUE(secondary_id, primary_id)
);
CREATE INDEX idx_relations_primary_id ON relations(primary_id);
CREATE INDEX idx_relations_secondary_id ON relations(secondary_id);
CREATE TABLE IF NOT EXISTS sessions (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    user_id BIGINT REFERENCES users(id) NOT NULL,
    master_token TEXT NOT NULL,
    device_token TEXT NOT NULL,
    device_info JSONB,
    ip_history INET[],
    current_secret TEXT,
    last_secret TEXT,
    last_jwt TEXT,
    tolerance_time TIMESTAMPTZ,
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    expires_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_sessions_user_device ON sessions(user_id, device_token);
CREATE TABLE IF NOT EXISTS user_settings (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    user_id BIGINT UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    privacy JSONB,
    notifications JSONB,
    language TEXT,
    theme SMALLINT NOT NULL,            -- Modifié : Plus de valeur par défaut
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);
CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);
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
    password_hash TEXT,
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
        u.password_hash,
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
        AND (p_username IS NULL OR TRIM(u.username) = TRIM(p_username))
        -- MODIFICATION ICI : On nettoie les espaces (TRIM) et on ignore la casse (LOWER)
        AND (p_email IS NULL OR TRIM(LOWER(u.email)) = TRIM(LOWER(p_email)))
        AND (p_phone IS NULL OR TRIM(u.phone) = TRIM(p_phone));
END;
$$ LANGUAGE plpgsql STABLE;
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
    p_master_token TEXT DEFAULT NULL
)
RETURNS TABLE (
    id BIGINT,
    user_id BIGINT,
    master_token TEXT,
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
            s.master_token,
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
        s.master_token,
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
        AND (p_master_token IS NULL OR s.master_token = p_master_token);
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

------------------------------------------------------------------------------

SET search_path TO content;
CREATE TABLE IF NOT EXISTS posts (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    user_id BIGINT REFERENCES auth.users(id) NOT NULL, -- Harmonisé vers 'users'
    content TEXT,
    media_ids BIGINT[],
    visibility SMALLINT,                -- Modifié : Plus de valeur par défaut (0)
    location TEXT,
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);
CREATE INDEX idx_posts_user_created ON posts(user_id, created_at DESC);
CREATE TABLE IF NOT EXISTS comments (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    post_id BIGINT REFERENCES posts(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES auth.users(id), -- Harmonisé vers 'users'
    content TEXT,
    visibility BOOLEAN,                 -- Modifié : Plus de valeur par défaut (TRUE)
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);
CREATE INDEX idx_comments_post_created ON comments(post_id, created_at DESC);
CREATE TABLE IF NOT EXISTS likes (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    target_type SMALLINT NOT NULL,
    target_id BIGINT NOT NULL,
    user_id BIGINT REFERENCES auth.users(id), -- Harmonisé vers 'users'
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    UNIQUE(target_type, target_id, user_id)
);
CREATE INDEX idx_likes_target ON likes(target_type, target_id);
CREATE TABLE IF NOT EXISTS media (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    owner_id BIGINT REFERENCES auth.users(id), -- Harmonisé vers 'users'
    storage_path TEXT,
    visibility BOOLEAN,                 -- Modifié : Plus de valeur par défaut (TRUE)
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);
CREATE INDEX idx_media_owner ON media(owner_id);
CREATE INDEX idx_media_created ON media(created_at);
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

------------------------------------------------------------------------------

SET search_path TO messaging;
CREATE TABLE IF NOT EXISTS conversations (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    type SMALLINT,
    title TEXT,                         -- Modifié : Plus de DEFAULT NULL
    last_message_id BIGINT UNIQUE,      -- Modifié : Plus de DEFAULT NULL
    last_read_by_all_message_id BIGINT, -- Modifié : Plus de DEFAULT NULL
    state SMALLINT,                     -- Modifié : Plus de valeur par défaut (0)
    laws SMALLINT[],
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);
CREATE INDEX idx_conversations_last_message ON conversations(last_message_id);
CREATE TABLE IF NOT EXISTS members (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    conversation_id BIGINT REFERENCES conversations(id), -- Corrigé : pointe vers 'conversations'
    user_id BIGINT REFERENCES auth.users(id), -- Harmonisé vers 'users'
    role SMALLINT,                      -- Modifié : Plus de valeur par défaut (0)
    joined_at TIMESTAMPTZ,              -- Modifié : Plus de DEFAULT now()
    unread_count INT,                   -- Modifié : Plus de valeur par défaut (0)
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    UNIQUE(conversation_id, user_id)    -- Note : Virgule ajoutée ici (absente dans l'original)
);
CREATE INDEX idx_members_conversation ON members(conversation_id);
CREATE INDEX idx_members_user ON members(user_id);
CREATE TABLE IF NOT EXISTS messages (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    conversation_id BIGINT REFERENCES conversations(id), -- Corrigé : pointe vers 'conversations'
    sender_id BIGINT NOT NULL,          -- Note : Pas de FK explicite ici dans l'original, gardé tel quel
    message_type SMALLINT NOT NULL,     -- Modifié : Plus de valeur par défaut (0)
    visibility BOOLEAN,                 -- Modifié : Plus de valeur par défaut (TRUE)
    content TEXT,
    attachments JSONB,
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);
CREATE INDEX idx_message_conv_created ON messages(conversation_id, created_at DESC);
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

------------------------------------------------------------------------------

SET search_path TO moderation;
CREATE TABLE IF NOT EXISTS reports (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur
    actor_id BIGINT REFERENCES auth.users(id), -- Harmonisé vers 'users'
    target_type SMALLINT NOT NULL,
    target_id BIGINT NOT NULL,
    reason TEXT,
    rationale TEXT,                     -- Modifié : Plus de DEFAULT NULL
    state SMALLINT,                     -- Modifié : Plus de valeur par défaut (0)
    created_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);
CREATE INDEX idx_reports_actor ON reports(actor_id);
CREATE INDEX idx_reports_created ON reports(created_at);
CREATE INDEX idx_reports_target ON reports(target_type, target_id);
CREATE INDEX idx_reports_state ON reports(state);
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