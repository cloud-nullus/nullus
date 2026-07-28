-- 자기 클러스터 등록(self-registration)을 위한 타입 추가.
--
-- Nullus 가 자기가 떠 있는 클러스터를 설치 대상으로 인식해야 하는 경우가 있다.
-- 에어갭 무인 설치와 단일 클러스터 배포 시나리오가 여기에 해당한다.
ALTER TYPE cluster_type ADD VALUE IF NOT EXISTS 'self';
