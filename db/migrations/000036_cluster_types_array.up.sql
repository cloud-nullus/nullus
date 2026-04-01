ALTER TABLE clusters
    ADD COLUMN IF NOT EXISTS types cluster_type[];

UPDATE clusters
SET types = ARRAY[type]::cluster_type[]
WHERE types IS NULL;

UPDATE clusters
SET type = 'pipeline',
    types = ARRAY['target', 'pipeline']::cluster_type[]
WHERE id IN (
    '31111111-1111-1111-1111-111111111111',
    '32222222-2222-2222-2222-222222222222',
    '35555555-5555-5555-5555-555555555555',
    '36666666-6666-6666-6666-666666666666',
    '37777777-7777-7777-7777-777777777777'
);
