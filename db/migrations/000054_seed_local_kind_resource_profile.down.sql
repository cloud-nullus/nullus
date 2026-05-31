DELETE FROM org_resource_profiles
WHERE org_id = '11111111-1111-1111-1111-111111111111'
  AND lower(name) = 'local kind'
  AND base_profile = 'local';
