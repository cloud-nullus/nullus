INSERT INTO org_members (org_id, user_id, role, joined_at)
SELECT u.org_id, u.id, u.role, u.created_at
FROM users u
WHERE u.org_id IS NOT NULL
ON CONFLICT (org_id, user_id) DO NOTHING;
