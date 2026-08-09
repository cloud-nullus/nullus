-- 000059 롤백: 레지스트리 기반 템플릿 2종을 제거한다.
--
-- 이 템플릿으로 만든 스택이 남아 있어도 스택 자체는 template_id 만 참조하므로
-- 삭제해도 동작 중인 스택에는 영향이 없다.

DELETE FROM compatibility_matrices WHERE id IN ('gitlab-harbor-v1', 'gitlab-nexus-v1');
DELETE FROM golden_path_templates  WHERE id IN ('gitlab-harbor-v1', 'gitlab-nexus-v1');
