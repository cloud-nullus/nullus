ALTER TABLE org_resource_profiles
    DROP CONSTRAINT IF EXISTS org_resource_profiles_base_profile_check;

UPDATE org_resource_profiles
SET base_profile = 'startup'
WHERE base_profile = 'local';

ALTER TABLE org_resource_profiles
    ADD CONSTRAINT org_resource_profiles_base_profile_check
    CHECK (base_profile IN ('startup', 'standard', 'enterprise'));
