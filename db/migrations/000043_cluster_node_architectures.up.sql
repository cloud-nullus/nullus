-- 000043_cluster_node_architectures.up.sql
-- Compatibility Matrix Task 3: clusters 테이블에 node_architectures TEXT[] 컬럼 추가.
--
-- 이 컬럼은 `node.status.nodeInfo.architecture` 의 distinct set 으로, admin 모듈의
-- ClusterUseCase.DiscoverCluster 결과를 기록한다. Stack 모듈의 Pre-Deploy Gate 가
-- ToolVersion.ArchSupport 와 이 값을 교차 검증해 ARM64-미지원 이미지가 ARM 노드가
-- 포함된 클러스터에 스케줄링되는 상황을 사전에 차단한다.
--
-- Idempotent: 동일 migrate up 을 반복해도 DDL 에러를 내지 않는다.

ALTER TABLE clusters
    ADD COLUMN IF NOT EXISTS node_architectures TEXT[] NOT NULL DEFAULT '{}';
