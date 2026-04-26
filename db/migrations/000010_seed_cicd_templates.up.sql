INSERT INTO pipeline_templates (
    id,
    name,
    description,
    app_type,
    stages
) VALUES
(
    'web-backend-v1',
    'Web Backend Pipeline',
    '백엔드 서비스를 위한 CI/CD 파이프라인. 빌드, 테스트, 이미지 빌드, 배포 단계를 포함합니다.',
    'backend',
    '["Build", "Test", "ImageBuild", "Deploy"]'::jsonb
),
(
    'web-frontend-v1',
    'Web Frontend Pipeline',
    '프론트엔드 서비스를 위한 CI/CD 파이프라인. 빌드, 테스트, 정적 빌드, 배포 단계를 포함합니다.',
    'web',
    '["Build", "Test", "StaticBuild", "Deploy"]'::jsonb
),
(
    'batch-job-v1',
    'Batch Job Pipeline',
    '배치 작업을 위한 CI/CD 파이프라인. 빌드, 이미지 빌드, CronJob 배포 단계를 포함합니다.',
    'batch',
    '["Build", "ImageBuild", "CronJobDeploy"]'::jsonb
);
