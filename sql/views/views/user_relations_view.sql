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