CREATE OR REPLACE VIEW views.post_engagement_view AS
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