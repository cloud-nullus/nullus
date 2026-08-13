-- 000063_seed_otel_stack_template.down.sql
-- 시드한 OpenTelemetry 템플릿과 매트릭스를 되돌린다.
--
-- 이 템플릿으로 만든 스택은 template_id 만 참조하므로(ON DELETE 제약 없음)
-- 시드 행만 지운다. 이미 배포된 스택의 설정은 stacks.config 에 복사되어 있어
-- 템플릿이 사라져도 그대로 동작한다.

DELETE FROM compatibility_matrices WHERE id = 'gitlab-argocd-otel-v1';
DELETE FROM golden_path_templates  WHERE id = 'gitlab-argocd-otel-v1';
