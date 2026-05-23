ALTER TABLE org_resource_profiles
    DROP CONSTRAINT IF EXISTS org_resource_profiles_base_profile_check;

ALTER TABLE org_resource_profiles
    ADD CONSTRAINT org_resource_profiles_base_profile_check
    CHECK (base_profile IN ('local', 'startup', 'standard', 'enterprise'));
