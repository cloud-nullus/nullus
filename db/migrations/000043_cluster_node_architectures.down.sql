-- 000043_cluster_node_architectures.down.sql
-- Task 3 의 node_architectures 컬럼을 idempotent 하게 제거.

ALTER TABLE clusters
    DROP COLUMN IF EXISTS node_architectures;
