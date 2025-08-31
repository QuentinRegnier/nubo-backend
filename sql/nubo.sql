-- Création des schémas
CREATE SCHEMA auth;
CREATE SCHEMA content;
CREATE SCHEMA messaging;
CREATE SCHEMA moderation;
CREATE SCHEMA views;
CREATE SCHEMA logic;

------------------------------------------------------------------------------

SET search_path TO auth;
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique de l'utilisateur
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
    profile_picture_id UUID, -- id de l'image de profil
    grade SMALLINT NOT NULL DEFAULT 1, -- grade de l'utilisateur
    location TEXT, -- localisation de l'utilisateur
    school TEXT, -- école
    work TEXT, -- emplois
    badges TEXT[], -- badges
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    updated_at TIMESTAMPTZ DEFAULT now() -- date de mise à jour
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);

CREATE TABLE IF NOT EXISTS user_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique des paramètres utilisateur
    user_id UUID UNIQUE REFERENCES users(id) ON DELETE CASCADE, -- id unique de l'utilisateur
    privacy JSONB, -- paramètres de confidentialité
    notifications JSONB, -- paramètres de notification
    language TEXT, -- langue
    theme SMALLINT NOT NULL DEFAULT 0 -- thème clair/sombre
);

CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique de la session
    user_id UUID REFERENCES users(id), -- id de l'utilisateur
    refresh_token TEXT, -- token de rafraîchissement
    device_info JSONB, -- informations sur l'/les appareil(s)
    ip INET[], -- adresse IP
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    expires_at TIMESTAMPTZ, -- date d'expiration
    revoked BOOLEAN DEFAULT FALSE -- session révoquée
);

CREATE INDEX idx_sessions_user_id_revoked ON sessions(user_id, revoked);

CREATE TABLE IF NOT EXISTS relations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du suivi
    primary_id UUID REFERENCES users(id), -- id de l'utilisateur qui suit
    secondary_id UUID REFERENCES users(id), -- id de l'utilisateur suivi
    state SMALLINT DEFAULT 1, -- état du suivi (2 = amis, 1 = suivi, 0 = inactif, -1 = bloqué)
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    UNIQUE(secondary_id, primary_id)
);

CREATE INDEX idx_relations_primary_id ON relations(primary_id);
CREATE INDEX idx_relations_secondary_id ON relations(secondary_id);

------------------------------------------------------------------------------

SET search_path TO content;
CREATE TABLE IF NOT EXISTS posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du post
    user_id UUID REFERENCES auth.users(id) NOT NULL, -- id de l'utilisateur
    content TEXT, -- contenu du post
    media_ids UUID[], -- ids des médias associés
    visibility SMALLINT DEFAULT 0, -- visibilité (1 = amis, 0 = public)
    location TEXT, -- localisation
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    updated_at TIMESTAMPTZ DEFAULT now() -- date de mise à jour
);

CREATE INDEX idx_posts_user_created ON posts(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du commentaire
    post_id UUID REFERENCES posts(id) ON DELETE CASCADE, -- id du post
    user_id UUID REFERENCES auth.users(id), -- id de l'utilisateur
    content TEXT, -- contenu du commentaire
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_comments_post_created ON comments(post_id, created_at DESC);

CREATE TABLE IF NOT EXISTS likes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du like
    target_type SMALLINT NOT NULL, -- type de la cible (0 = post, 1 = message, 2 = commentaire)
    target_id UUID NOT NULL, -- id de la cible
    user_id UUID REFERENCES auth.users(id), -- id de l'utilisateur
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    UNIQUE(target_type, target_id, user_id)
);

CREATE INDEX idx_likes_target ON likes(target_type, target_id);

CREATE TABLE IF NOT EXISTS media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du média
    owner_id UUID REFERENCES auth.users(id), -- id du propriétaire
    storage_path TEXT, -- chemin de stockage
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_media_owner ON media(owner_id);
CREATE INDEX idx_media_created ON media(created_at);

------------------------------------------------------------------------------

SET search_path TO moderation;
CREATE TABLE IF NOT EXISTS reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du rapport
    actor_id UUID REFERENCES auth.users(id), -- id de l'utilisateur ayant signalé
    target_type SMALLINT NOT NULL, -- type de la cible (user/post/comment/etc)
    target_id UUID NOT NULL, -- id de la cible
    reason TEXT, -- raison du signalement
    state SMALLINT DEFAULT 0, -- état du rapport (0=pending, 1=reviewed, 2=resolved)
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_reports_actor ON reports(actor_id);
CREATE INDEX idx_reports_created ON reports(created_at);

------------------------------------------------------------------------------

SET search_path TO messaging;
CREATE TABLE IF NOT EXISTS conversations_meta (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique de la conversation
    type SMALLINT, -- type de la conversation (0 = message privée, 1 = groupe, 2 = communauté, 3 = annonce)
    title TEXT, -- titre de la conversation
    last_message_id UUID UNIQUE, -- id du dernier message
    state SMALLINT DEFAULT 0, -- état de la conversation (0 = active, 1 = supprimée, 2 = archivée)
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_conversations_last_message ON conversations_meta(last_message_id);

CREATE TABLE IF NOT EXISTS conversation_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du membre
    conversation_id UUID REFERENCES conversations_meta(id), -- id de la conversation
    user_id UUID REFERENCES auth.users(id), -- id de l'utilisateur
    role SMALLINT DEFAULT 0, -- rôle du membre (0 = membre, 1 = admin, 2 = créateur)
    joined_at TIMESTAMPTZ DEFAULT now(), -- date d'adhésion
    unread_count INT DEFAULT 0, -- nombre de messages non lus
    UNIQUE(conversation_id, user_id)
);

CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique du message
    conversation_id UUID REFERENCES conversations_meta(id), -- id de la conversation
    sender_id UUID NOT NULL, -- id de l'expéditeur
    message_type SMALLINT NOT NULL DEFAULT 0, -- 0=text, 1=image, 2=publication, 3=vocal, 4=vidéo
    state SMALLINT NOT NULL DEFAULT 0, -- 0=actif, 1=supprimé
    content TEXT, -- contenu du message
    attachments JSONB, -- pointeurs vers fichiers S3 / metadata
    created_at TIMESTAMPTZ DEFAULT now() -- date de création
);

CREATE INDEX idx_message_conv_created ON messages(conversation_id, created_at DESC);

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
    last_msg.state AS last_message_state,
    last_msg.content AS last_message_content,
    last_msg.created_at AS last_message_time
FROM messaging.conversation_members cm
LEFT JOIN LATERAL (
    SELECT m.*
    FROM messaging.messages m
    WHERE m.conversation_id = cm.conversation_id
    ORDER BY m.created_at DESC
    LIMIT 1
) last_msg ON true;
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

------------------------------------------------------------------------------

SET search_path TO logic;
CREATE OR REPLACE FUNCTION func_create_comment(
    p_post_id UUID,
    p_user_id UUID,
    p_content TEXT
) RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_comment_id UUID;
BEGIN
    INSERT INTO content.comments(post_id, user_id, content)
    VALUES (p_post_id, p_user_id, p_content)
    RETURNING id INTO v_comment_id;

    RETURN v_comment_id;
END;
$$;
CREATE OR REPLACE FUNCTION messaging.func_create_conversation(
    p_type SMALLINT,
    p_title TEXT,
    p_members JSONB -- tableau JSON du type [{ "user_id": "...", "role": 2}, ...]
) RETURNS UUID AS $$
DECLARE
    v_conversation_id UUID;
    v_member JSONB;
BEGIN
    -- 1️⃣ Conversation meta
    INSERT INTO messaging.conversations_meta (type, title)
    VALUES (p_type, p_title)
    RETURNING id INTO v_conversation_id;

    -- 2️⃣ Ajout des membres
    FOR v_member IN SELECT * FROM jsonb_array_elements(p_members)
    LOOP
        INSERT INTO messaging.conversation_members (conversation_id, user_id, role)
        VALUES (
            v_conversation_id,
            (v_member->>'user_id')::UUID,
            COALESCE((v_member->>'role')::SMALLINT, 0)
        );
    END LOOP;

    RETURN v_conversation_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION func_create_media(
    p_owner_id UUID,
    p_storage_path TEXT
) RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_media_id UUID;
BEGIN
    INSERT INTO content.media(owner_id, storage_path)
    VALUES (p_owner_id, p_storage_path)
    RETURNING id INTO v_media_id;

    RETURN v_media_id;
END;
$$;
CREATE OR REPLACE FUNCTION func_create_message(
    p_conversation_id UUID,
    p_sender_id UUID,
    p_content TEXT,
    p_attachments JSONB,
    p_message_type SMALLINT DEFAULT 0,
    p_state SMALLINT DEFAULT 0
) RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_message_id UUID;
BEGIN
    -- Insérer message
    INSERT INTO messaging.messages(conversation_id, sender_id, message_type, state, content, attachments)
    VALUES (p_conversation_id, p_sender_id, p_message_type, p_state, p_content, p_attachments)
    RETURNING id INTO v_message_id;

    -- Mettre à jour la conversation
    UPDATE messaging.conversations_meta
    SET last_message_id = v_message_id
    WHERE id = p_conversation_id;

    -- Incrémenter unread_count
    UPDATE messaging.conversation_members
    SET unread_count = unread_count + 1
    WHERE conversation_id = p_conversation_id
      AND user_id <> p_sender_id;

    RETURN v_message_id;
END;
$$;
CREATE OR REPLACE FUNCTION func_create_post(
    p_user_id UUID,
    p_content TEXT,
    p_media_ids UUID[],
    p_visibility SMALLINT,
    p_location TEXT
) RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_post_id UUID;
BEGIN
    INSERT INTO content.posts(user_id, content, media_ids, visibility, location)
    VALUES (p_user_id, p_content, p_media_ids, p_visibility, p_location)
    RETURNING id INTO v_post_id;

    RETURN v_post_id;
END;
$$;
CREATE OR REPLACE FUNCTION func_create_user_conversation(
    p_conversation_id UUID,
    p_user_id UUID,
    p_role SMALLINT DEFAULT 0
) RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_member_id UUID;
BEGIN
    INSERT INTO messaging.conversation_members(conversation_id, user_id, role)
    VALUES (p_conversation_id, p_user_id, p_role)
    ON CONFLICT (conversation_id, user_id) DO UPDATE
        SET role = EXCLUDED.role
    RETURNING id INTO v_member_id;

    RETURN v_member_id;
END;
$$;
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
    p_profile_picture_id UUID,
    p_location TEXT,
    p_school TEXT,
    p_work TEXT,
    p_badges TEXT[],
    p_privacy JSONB,
    p_notifications JSONB,
    p_language TEXT,
    p_theme TEXT,
    p_refresh_token TEXT,
    p_device_info JSONB,
    p_ip INET[],
    p_expires_at TIMESTAMPTZ
) RETURNS UUID AS $$
DECLARE
    v_user_id UUID;
BEGIN
    -- 1️⃣ Créer l'utilisateur
    INSERT INTO auth.users (
        username, email, phone, password_hash, first_name, last_name,
        birthdate, sex, bio, profile_picture_id, location, school, work, badges
    ) VALUES (
        p_username, p_email, p_phone, p_password_hash, p_first_name, p_last_name,
        p_birthdate, p_sex, p_bio, p_profile_picture_id, p_location, p_school, p_work, p_badges
    )
    RETURNING id INTO v_user_id;

    -- 2️⃣ Créer les settings associés
    INSERT INTO auth.user_settings (user_id, privacy, notifications, language, theme)
    VALUES (v_user_id, p_privacy, p_notifications, p_language, p_theme);

    -- 3️⃣ Créer la session
    INSERT INTO auth.sessions (user_id, refresh_token, device_info, ip, expires_at)
    VALUES (v_user_id, p_refresh_token, p_device_info, p_ip, p_expires_at);

    RETURN v_user_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION content.func_load_comments(
    p_post_id UUID,
    p_limit INT,
    p_order_mode SMALLINT DEFAULT 0 -- 0=recent, 1=most liked
)
RETURNS TABLE(
    id UUID,
    content TEXT,
    user_id UUID,
    created_at TIMESTAMPTZ,
    like_count INT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT c.id, c.content, c.user_id, c.created_at,
           (SELECT COUNT(*) FROM content.likes l WHERE l.target_type=2 AND l.target_id=c.id) AS like_count
    FROM content.comments c
    WHERE c.post_id = p_post_id
    ORDER BY
        CASE WHEN p_order_mode=0 THEN c.created_at END DESC,
        CASE WHEN p_order_mode=1 THEN (SELECT COUNT(*) FROM content.likes l WHERE l.target_type=2 AND l.target_id=c.id) END DESC
    LIMIT p_limit;
END;
$$;

CREATE OR REPLACE FUNCTION func_load_messages(
    p_conversation_id UUID
) RETURNS SETOF messaging.messages
LANGUAGE sql
AS $$
    SELECT *
    FROM messaging.messages
    WHERE conversation_id = p_conversation_id
    ORDER BY created_at ASC;
$$;
CREATE OR REPLACE FUNCTION func_get_user_posts(
    p_user_id UUID
) RETURNS SETOF content.posts
LANGUAGE sql
AS $$
    SELECT *
    FROM content.posts
    WHERE user_id = p_user_id
    ORDER BY created_at DESC;
$$;

------------------------------------------------------------------------------

SET search_path TO logic;
CREATE OR REPLACE PROCEDURE proc_add_like(
    p_target_type SMALLINT,
    p_target_id UUID,
    p_user_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO content.likes(target_type, target_id, user_id)
    VALUES (p_target_type, p_target_id, p_user_id)
    ON CONFLICT (target_type, target_id, user_id) DO NOTHING;
END;
$$;
CREATE OR REPLACE PROCEDURE proc_create_relation(
    p_primary_id UUID,
    p_secondary_id UUID,
    p_state SMALLINT DEFAULT 1
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO auth.relations(primary_id, secondary_id, state)
    VALUES (p_primary_id, p_secondary_id, p_state)
    ON CONFLICT (primary_id, secondary_id) DO UPDATE
    SET state = excluded.state;
END;
$$;

CREATE OR REPLACE PROCEDURE proc_create_report(
    p_actor_id UUID,
    p_target_type SMALLINT,
    p_target_id UUID,
    p_reason TEXT,
    p_state SMALLINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO moderation.reports(actor_id, target_type, target_id, reason, state)
    VALUES (p_actor_id, p_target_type, p_target_id, p_reason, p_state);
END;
$$;

CREATE OR REPLACE PROCEDURE proc_delete_comment(
    p_comment_id UUID,
    p_user_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM content.comments
    WHERE id = p_comment_id AND user_id = p_user_id;
END;
$$;

CREATE OR REPLACE PROCEDURE proc_delete_conversation(
    p_conversation_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE messaging.conversations_meta
    SET state = 1
    WHERE id = p_conversation_id;
END;
$$;

CREATE OR REPLACE PROCEDURE proc_remove_like(
    p_target_type SMALLINT,
    p_target_id UUID,
    p_user_id UUID
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
CREATE OR REPLACE PROCEDURE proc_delete_media(
    p_media_id UUID,
    p_owner_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM content.media
    WHERE id = p_media_id AND owner_id = p_owner_id;
END;
$$;

CREATE OR REPLACE PROCEDURE proc_delete_message(
    p_message_id UUID,
    p_user_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE messaging.messages
    SET state = 1
    WHERE id = p_message_id
      AND sender_id = p_user_id
      AND state = 0;
END;
$$;

CREATE OR REPLACE PROCEDURE proc_delete_relation(
    p_user_id UUID,
    p_target_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM auth.relations
    WHERE (follower_id = p_user_id AND followed_id = p_target_id)
       OR (follower_id = p_target_id AND followed_id = p_user_id);
END;
$$;
CREATE OR REPLACE PROCEDURE proc_delete_user_conversation(
    p_conversation_id UUID,
    p_user_id UUID
)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM messaging.conversation_members
    WHERE conversation_id = p_conversation_id AND user_id = p_user_id;
END;
$$;

CREATE OR REPLACE PROCEDURE proc_update_comment(
    p_comment_id UUID,
    p_user_id UUID,
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

CREATE OR REPLACE PROCEDURE proc_update_message(
    p_message_id UUID,
    p_user_id UUID,
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
      AND state = 0;       -- actif
END;
$$;

CREATE OR REPLACE PROCEDURE proc_update_post(
    p_post_id UUID,
    p_user_id UUID,
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

CREATE OR REPLACE PROCEDURE proc_update_relation(
    p_primary_id UUID,
    p_secondary_id UUID,
    p_state SMALLINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE auth.relations
    SET state = p_state
    WHERE primary_id = p_primary_id AND secondary_id = p_secondary_id;
END;
$$;

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

CREATE OR REPLACE PROCEDURE proc_update_user_role_conversation(
    p_conversation_id UUID,
    p_user_id UUID,
    p_role SMALLINT
)
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE conversation_members
    SET role = p_role
    WHERE conversation_id = p_conversation_id AND user_id = p_user_id;
END;
$$;
