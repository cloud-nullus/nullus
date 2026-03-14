ALTER TABLE organizations DROP CONSTRAINT IF EXISTS fk_organizations_default_admin;

DROP TABLE IF EXISTS clusters;
DROP TABLE IF EXISTS org_members;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;

DROP TYPE IF EXISTS cluster_type;
DROP TYPE IF EXISTS connection_status;
DROP TYPE IF EXISTS user_role;
DROP TYPE IF EXISTS org_status;
