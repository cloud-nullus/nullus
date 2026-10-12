-- 000084 롤백: GitLab + Harbor + Trivy 템플릿을 제거한다.
--
-- 이 템플릿으로 만든 스택이 남아 있어도 스택은 template_id 만 참조하므로
-- 삭제해도 동작 중인 스택에는 영향이 없다.

DELETE FROM compatibility_matrices WHERE id = 'gitlab-harbor-trivy-v1';
DELETE FROM golden_path_templates  WHERE id = 'gitlab-harbor-trivy-v1';
