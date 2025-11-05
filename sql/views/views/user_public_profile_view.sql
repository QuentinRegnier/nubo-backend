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