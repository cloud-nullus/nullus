UPDATE pipeline_templates
SET name = 'Web Backend Pipeline'
WHERE id = 'web-backend-v1';

INSERT INTO pipeline_templates (id, name, description, app_type, stages)
VALUES (
    'web-frontend-v1',
    'Web Frontend Pipeline',
    '프론트엔드 서비스를 위한 CI/CD 파이프라인. 빌드, 테스트, 정적 빌드, 배포 단계를 포함합니다.',
    'web',
    '["Build", "Test", "StaticBuild", "Deploy"]'::jsonb
)
ON CONFLICT (id) DO NOTHING;
