// 문서 캡처용 고정 응답.
//
// 실제 클러스터를 띄우지 않고도 "데이터가 있는 화면" 을 찍기 위한 것이다.
// 빈 목록을 찍으면 문서 그림으로 쓸 수 없고, 살아 있는 환경을 찍으면 매번 다른
// 그림(IP · 토큰 · 타임스탬프)이 나와 재현이 안 된다.
//
// 값은 db/migrations 의 시드와 같은 모양으로 맞췄다. 정규화기(stack-normalizers.ts)가
// camelCase 와 snake_case 를 모두 받으므로 읽기 쉬운 쪽으로 적는다.

const ORG_ID = '00000000-0000-0000-0000-000000000001'
const CLUSTER_PLATFORM = 'cls_platform_0001'
const CLUSTER_DEVELOP = 'cls_develop_0001'

export const STACK_TEMPLATES = [
  {
    id: 'gitlab-argocd-sonarqube-v1',
    name: 'GitLab + Argo CD + SonarQube',
    description: 'GitLab CI 로 빌드하고 Argo CD 로 배포한다. SonarQube 정적 분석과 Trivy 이미지 스캔이 파이프라인에 붙는다.',
    category: 'recommended',
    estimatedMinutes: 135,
    recommendedUseCase: '보안 게이트가 필요한 조직',
    minResources: '12 vCPU / 24Gi RAM / 160Gi Storage',
    planningProfile: 'standard',
    tools: [
      { category: 'source_repository', name: 'GitLab CE', helm_version: '8.7.2', app_version: '17.7.2' },
      { category: 'ci_platform', name: 'GitLab CI', helm_version: '8.7.2', app_version: '17.7.2' },
      { category: 'container_registry', name: 'Harbor', helm_version: '1.14.0', app_version: '2.11.0' },
      { category: 'cd_tool', name: 'Argo CD', helm_version: '7.7.16', app_version: '2.13.2' },
      { category: 'sast', name: 'SonarQube', helm_version: '2026.5.1002', app_version: '2026.5' },
      { category: 'image_scanner', name: 'Trivy', helm_version: '0.26.0', app_version: '0.58.1' },
      { category: 'monitoring_collection', name: 'Prometheus', helm_version: '67.0.0', app_version: '3.1.0' },
      { category: 'monitoring_visualization', name: 'Grafana', helm_version: '8.5.0', app_version: '11.4.0' },
    ],
  },
  {
    id: 'gitlab-argocd-v1',
    name: 'GitLab + Argo CD',
    description: 'GitLab CI 와 Harbor 레지스트리를 분리해 GitOps 패턴을 강화한 구성.',
    category: 'recommended',
    estimatedMinutes: 120,
    recommendedUseCase: 'GitOps 중심 조직',
    minResources: '10 vCPU / 20Gi RAM / 130Gi Storage',
    planningProfile: 'standard',
    tools: [
      { category: 'source_repository', name: 'GitLab CE', helm_version: '8.7.2', app_version: '17.7.2' },
      { category: 'ci_platform', name: 'GitLab CI', helm_version: '8.7.2', app_version: '17.7.2' },
      { category: 'container_registry', name: 'Harbor', helm_version: '1.14.0', app_version: '2.11.0' },
      { category: 'cd_tool', name: 'Argo CD', helm_version: '7.7.16', app_version: '2.13.2' },
      { category: 'monitoring_collection', name: 'Prometheus', helm_version: '67.0.0', app_version: '3.1.0' },
      { category: 'monitoring_visualization', name: 'Grafana', helm_version: '8.5.0', app_version: '11.4.0' },
    ],
  },
  {
    id: 'gitea-jenkins-argocd-lite-v1',
    name: 'Gitea + Jenkins + Argo CD (Lite)',
    description: '8Gi 노드 하나에 들어가는 경량 구성. 로컬 검증과 소규모 팀에 쓴다.',
    category: 'lite',
    estimatedMinutes: 45,
    recommendedUseCase: '로컬 검증 · 소규모 팀',
    minResources: '4 vCPU / 8Gi RAM / 50Gi Storage',
    planningProfile: 'local',
    tools: [
      { category: 'source_repository', name: 'Gitea', helm_version: '10.6.0', app_version: '1.23.1' },
      { category: 'ci_platform', name: 'Jenkins', helm_version: '5.8.18', app_version: '2.568.3' },
      { category: 'cd_tool', name: 'Argo CD', helm_version: '7.7.16', app_version: '2.13.2' },
      { category: 'monitoring_collection', name: 'Prometheus', helm_version: '67.0.0', app_version: '3.1.0' },
    ],
  },
  {
    id: 'github-argocd-v1',
    name: 'GitHub + Argo CD',
    description: 'GitHub · GitHub Actions · GHCR 을 외부 서비스로 쓰고 클러스터에는 Argo CD 와 모니터링만 설치한다.',
    category: 'external',
    estimatedMinutes: 45,
    recommendedUseCase: 'GitHub 사용 조직',
    minResources: '4 vCPU / 8Gi RAM / 50Gi Storage',
    planningProfile: 'local',
    tools: [
      { category: 'source_repository', name: 'GitHub', helm_version: 'external', app_version: 'external' },
      { category: 'ci_platform', name: 'GitHub Actions', helm_version: 'external', app_version: 'external' },
      { category: 'container_registry', name: 'GHCR', helm_version: 'external', app_version: 'external' },
      { category: 'cd_tool', name: 'Argo CD', helm_version: '7.7.16', app_version: '2.13.2' },
      { category: 'monitoring_collection', name: 'Prometheus', helm_version: '67.0.0', app_version: '3.1.0' },
    ],
  },
]

export const STACKS = [
  {
    id: 'stk_7a31c0',
    name: 'nullus-gitlab-argocd-sonarqube',
    template_id: 'gitlab-argocd-sonarqube-v1',
    template_name: 'GitLab + Argo CD + SonarQube',
    cluster_id: CLUSTER_PLATFORM,
    cluster_name: 'kind-nullus-platform',
    namespace: 'nullus-gitlab-argocd-sonarqube',
    state: 'completed',
    status: 'running',
    org_id: ORG_ID,
    created_at: '2026-10-12T02:10:00Z',
    updated_at: '2026-10-12T04:25:00Z',
  },
  {
    id: 'stk_41b9de',
    name: 'nullus-lite',
    template_id: 'gitea-jenkins-argocd-lite-v1',
    template_name: 'Gitea + Jenkins + Argo CD (Lite)',
    cluster_id: CLUSTER_DEVELOP,
    cluster_name: 'kind-nullus-develop',
    namespace: 'nullus-lite',
    state: 'completed',
    status: 'running',
    org_id: ORG_ID,
    created_at: '2026-10-09T23:40:00Z',
    updated_at: '2026-10-10T00:28:00Z',
  },
]

export const CICD_TEMPLATES = [
  {
    id: 'web-frontend-react-v1',
    name: 'React 프론트엔드',
    description: 'Vite 빌드 → 이미지 → Argo CD 배포. 시크릿 탐지와 이미지 스캔이 붙는다.',
    app_type: 'web',
    stages: ['SecretScan', 'Build', 'ImageScan', 'Deploy'],
    git_repo_url: 'https://gitlab.nullus.local/apps/web-frontend',
    dockerfile_path: 'Dockerfile',
    docker_context: './',
    created_by: 'admin@nullus.dev',
  },
  {
    id: 'user-custom-pipeline',
    name: 'User Custom Pipeline',
    description: '빌드 설정을 직접 채우는 빈 템플릿.',
    app_type: 'backend',
    stages: ['SecretScan', 'Build', 'SAST', 'ImageScan', 'Deploy'],
    git_repo_url: '',
    dockerfile_path: 'Dockerfile',
    docker_context: './',
    created_by: 'admin@nullus.dev',
  },
  {
    id: 'batch-job-v1',
    name: '배치 작업',
    description: 'CronJob 으로 배포되는 배치 파이프라인.',
    app_type: 'batch',
    stages: ['SecretScan', 'Build', 'Deploy'],
    git_repo_url: '',
    dockerfile_path: 'Dockerfile',
    docker_context: './',
    created_by: 'admin@nullus.dev',
  },
]

export const PIPELINES = [
  {
    id: 'pl_3f9a21',
    name: 'web-frontend',
    mode: 'ci_cd',
    app_type: 'web',
    template_id: 'web-frontend-react-v1',
    stack_id: 'stk_7a31c0',
    execution_mode: 'stack_integrated',
    git_repo_url: 'https://gitlab.nullus.local/apps/web-frontend',
    access_url: 'https://web-frontend.nullus.local',
    cluster_id: CLUSTER_DEVELOP,
    cluster_name: 'kind-nullus-develop',
    namespace: 'nullus-app',
    status: 'active',
    stages: ['SecretScan', 'Build', 'ImageScan', 'Deploy'],
    last_deployed_at: '2026-10-12T06:41:00Z',
    created_at: '2026-10-12T05:02:00Z',
  },
  {
    id: 'pl_8c2b74',
    name: 'order-api',
    mode: 'ci_cd',
    app_type: 'backend',
    template_id: 'user-custom-pipeline',
    stack_id: 'stk_7a31c0',
    execution_mode: 'stack_integrated',
    git_repo_url: 'https://gitlab.nullus.local/apps/order-api',
    access_url: 'https://order-api.nullus.local',
    cluster_id: CLUSTER_DEVELOP,
    cluster_name: 'kind-nullus-develop',
    namespace: 'nullus-app',
    status: 'active',
    stages: ['SecretScan', 'Build', 'SAST', 'ImageScan', 'Deploy'],
    last_deployed_at: '2026-10-11T22:14:00Z',
    created_at: '2026-10-11T21:30:00Z',
  },
]

export const DEPLOYMENTS = [
  {
    id: 'dep_91',
    pipeline_id: 'pl_3f9a21',
    pipeline_name: 'web-frontend',
    version: 'v12',
    status: 'success',
    deployed_by: 'developer@nullus.dev',
    deployed_at: '2026-10-12T06:41:00Z',
    duration_seconds: 184,
  },
  {
    id: 'dep_90',
    pipeline_id: 'pl_8c2b74',
    pipeline_name: 'order-api',
    version: 'v7',
    status: 'failed',
    deployed_by: 'developer@nullus.dev',
    deployed_at: '2026-10-11T22:14:00Z',
    duration_seconds: 96,
    error: 'image-scan: CRITICAL 1건 (CVE-2021-44906)',
  },
  {
    id: 'dep_89',
    pipeline_id: 'pl_3f9a21',
    pipeline_name: 'web-frontend',
    version: 'v11',
    status: 'success',
    deployed_by: 'developer@nullus.dev',
    deployed_at: '2026-10-11T08:02:00Z',
    duration_seconds: 171,
  },
]

export const GOLDEN_PATHS = [
  {
    id: 'gitlab-allinone-v1',
    name: 'GitLab All-in-One',
    description: 'GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.',
    estimated_install_time: 90,
    recommended_use_case: '중견기업, 단일 플랫폼 선호',
    min_resources: '8 vCPU / 16Gi RAM / 100Gi Storage',
    created_at: '2026-10-01T00:00:00Z',
    tools: [
      { category: 'source_repository', name: 'GitLab CE', helm_version: '8.7.2', app_version: '17.7.2' },
      { category: 'ci_platform', name: 'GitLab CI', helm_version: '8.7.2', app_version: '17.7.2' },
      { category: 'container_registry', name: 'GitLab Registry', helm_version: '8.7.2', app_version: '17.7.2' },
      { category: 'cd_tool', name: 'Argo CD', helm_version: '7.7.2', app_version: '2.13.2' },
      { category: 'monitoring_collection', name: 'Prometheus', helm_version: '67.0.0', app_version: '3.1.0' },
      { category: 'monitoring_visualization', name: 'Grafana', helm_version: '8.5.0', app_version: '11.4.0' },
      { category: 'log_aggregation', name: 'Loki', helm_version: '6.6.2', app_version: '3.0.0' },
    ],
  },
  {
    id: 'gitlab-argocd-v1',
    name: 'GitLab + Argo CD',
    description: 'GitLab CI와 Harbor 레지스트리를 분리하여 GitOps 패턴을 강화한 구성입니다.',
    estimated_install_time: 120,
    recommended_use_case: 'GitOps 중심 조직',
    min_resources: '10 vCPU / 20Gi RAM / 130Gi Storage',
    created_at: '2026-10-01T00:00:00Z',
    tools: [
      { category: 'source_repository', name: 'GitLab CE', helm_version: '8.7.2', app_version: '17.7.2' },
      { category: 'ci_platform', name: 'GitLab CI', helm_version: '8.7.2', app_version: '17.7.2' },
      { category: 'container_registry', name: 'Harbor', helm_version: '1.14.0', app_version: '2.11.0' },
      { category: 'cd_tool', name: 'Argo CD', helm_version: '7.7.2', app_version: '2.13.2' },
      { category: 'monitoring_collection', name: 'Prometheus', helm_version: '67.0.0', app_version: '3.1.0' },
      { category: 'monitoring_visualization', name: 'Grafana', helm_version: '8.5.0', app_version: '11.4.0' },
      { category: 'log_aggregation', name: 'Loki', helm_version: '6.6.2', app_version: '3.0.0' },
    ],
  },
  {
    id: 'github-argocd-v1',
    name: 'GitHub + Argo CD',
    description: 'GitHub·GitHub Actions·GHCR 을 외부 서비스로 사용하고, 클러스터 내에는 Argo CD + 모니터링만 설치합니다.',
    estimated_install_time: 45,
    recommended_use_case: 'GitHub 사용 조직',
    min_resources: '4 vCPU / 8Gi RAM / 50Gi Storage',
    created_at: '2026-10-01T00:00:00Z',
    tools: [
      { category: 'source_repository', name: 'GitHub', helm_version: 'external', app_version: 'external' },
      { category: 'ci_platform', name: 'GitHub Actions', helm_version: 'external', app_version: 'external' },
      { category: 'container_registry', name: 'GHCR', helm_version: 'external', app_version: 'external' },
      { category: 'cd_tool', name: 'Argo CD', helm_version: '7.7.2', app_version: '2.13.2' },
      { category: 'monitoring_collection', name: 'Prometheus', helm_version: '67.0.0', app_version: '3.1.0' },
      { category: 'monitoring_visualization', name: 'Grafana', helm_version: '8.5.0', app_version: '11.4.0' },
    ],
  },
]

export const CLUSTERS = [
  {
    id: CLUSTER_PLATFORM,
    name: 'kind-nullus-platform',
    type: 'pipeline',
    types: ['pipeline'],
    cloud_provider: 'on_premise',
    endpoint: 'https://127.0.0.1:6443',
    connection_status: 'connected',
    node_architectures: ['amd64'],
    org_id: ORG_ID,
    created_at: '2026-10-08T01:00:00Z',
  },
  {
    id: CLUSTER_DEVELOP,
    name: 'kind-nullus-develop',
    type: 'target',
    types: ['target'],
    cloud_provider: 'on_premise',
    endpoint: 'https://127.0.0.1:6445',
    connection_status: 'connected',
    node_architectures: ['amd64'],
    org_id: ORG_ID,
    created_at: '2026-10-08T01:04:00Z',
  },
]

export const MEMBERS = [
  { id: 'mem_1', name: 'Admin', email: 'admin@nullus.dev', role: 'admin', status: 'active', joined_at: '2026-09-01T00:00:00Z', is_active: true },
  { id: 'mem_2', name: 'DevOps', email: 'devops@nullus.dev', role: 'devops', status: 'active', joined_at: '2026-09-02T00:00:00Z', is_active: true },
  { id: 'mem_3', name: 'Developer', email: 'developer@nullus.dev', role: 'developer', status: 'active', joined_at: '2026-09-03T00:00:00Z', is_active: true },
  { id: 'mem_4', name: '이소연', email: 'soyeon@nullus.dev', role: 'developer', status: 'inactive', joined_at: '2026-10-05T00:00:00Z', is_active: false },
]

export const ORGANIZATION = {
  id: ORG_ID,
  name: 'Nullus 팀',
  slug: 'nullus-team',
  domain: 'nullus.dev',
  cluster_access_scope: 'organization',
  status: 'active',
  created_at: '2026-09-01T00:00:00Z',
}

export const STACK_ALERTS = [
  {
    id: 'al_31',
    stack_id: 'stk_7a31c0',
    stack_name: 'nullus-gitlab-argocd-sonarqube',
    alertname: 'KubePodCrashLooping',
    severity: 'critical',
    active: true,
    summary: 'Pod nullus-app/order-api-6f8d9 가 5분 동안 재시작을 반복하고 있다',
    runbook_url: 'https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubepodcrashlooping',
    labels: { namespace: 'nullus-app', pod: 'order-api-6f8d9' },
    started_at: '2026-10-12T07:12:00Z',
  },
  {
    id: 'al_30',
    stack_id: 'stk_7a31c0',
    stack_name: 'nullus-gitlab-argocd-sonarqube',
    alertname: 'KubePersistentVolumeFillingUp',
    severity: 'critical',
    active: false,
    summary: 'PVC gitlab-postgresql 의 남은 용량이 12% 다',
    runbook_url: 'https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubepersistentvolumefillingup',
    labels: { namespace: 'nullus-gitlab-argocd-sonarqube', persistentvolumeclaim: 'gitlab-postgresql' },
    started_at: '2026-10-11T19:40:00Z',
    resolved_at: '2026-10-11T20:55:00Z',
  },
  {
    id: 'al_29',
    stack_id: 'stk_41b9de',
    stack_name: 'nullus-lite',
    alertname: 'TargetDown',
    severity: 'critical',
    active: false,
    summary: 'jenkins 수집 대상이 응답하지 않는다',
    labels: { namespace: 'nullus-lite', job: 'jenkins' },
    started_at: '2026-10-10T11:02:00Z',
    resolved_at: '2026-10-10T11:19:00Z',
  },
]

export const NOTIFICATION_CHANNELS = [
  {
    id: 'ch_1',
    name: '플랫폼 운영 Slack',
    channel: 'slack',
    is_active: true,
    config: { channel: '#nullus-ops' },
    secret_hint: 'https://hooks.slack.com/services/T***',
    events: ['backup_failed', 'backup_partial', 'alert_fired', 'alert_resolved'],
    created_at: '2026-10-02T00:00:00Z',
  },
  {
    id: 'ch_2',
    name: '당직 Telegram',
    channel: 'telegram',
    is_active: true,
    config: { chat_id: '-1001234567890' },
    secret_hint: '7612****:AAG***',
    events: ['backup_failed', 'alert_fired'],
    created_at: '2026-10-02T00:00:00Z',
  },
]

export const NOTIFICATION_EVENTS = [
  { type: 'backup_failed', label: '백업 실패', severity: 'critical', default_enabled: true },
  { type: 'backup_partial', label: '백업 부분 성공', severity: 'warning', default_enabled: true },
  { type: 'backup_succeeded', label: '백업 성공', severity: 'info', default_enabled: false },
  { type: 'alert_fired', label: '경보 발생', severity: 'critical', default_enabled: true },
  { type: 'alert_resolved', label: '경보 해제', severity: 'info', default_enabled: true },
]

export const NOTIFICATION_HISTORY = [
  { id: 'nh_8', channel_id: 'ch_1', channel: 'slack', event_type: 'alert_fired', status: 'sent', attempts: 1, created_at: '2026-10-12T07:12:04Z' },
  { id: 'nh_7', channel_id: 'ch_2', channel: 'telegram', event_type: 'alert_fired', status: 'sent', attempts: 1, created_at: '2026-10-12T07:12:03Z' },
  { id: 'nh_6', channel_id: 'ch_1', channel: 'slack', event_type: 'backup_partial', status: 'failed', attempts: 3, error: 'webhook 410 Gone', created_at: '2026-10-11T18:00:12Z' },
]

export const APP_TEMPLATES = [
  { id: 'nullus-sample-frontend-v1', name: '샘플 프론트엔드', app_type: 'web', description: 'Vite + React 샘플 앱' },
  { id: 'nullus-sample-backend-v1', name: '샘플 백엔드', app_type: 'backend', description: 'Go + Echo 샘플 API' },
]

export const KNOWN_ISSUES = [
  { id: 'ki_1', title: '스택 이력의 롤백이 400 을 반환한다', severity: 'high', status: 'open', component: 'stack', created_at: '2026-10-05T00:00:00Z' },
  { id: 'ki_2', title: '호환성 매트릭스 생성·수정·삭제가 404 다', severity: 'medium', status: 'open', component: 'stack', created_at: '2026-10-05T00:00:00Z' },
]

export const RESOURCE_DEFAULTS = [
  { tool_key: 'gitlab', cpu_request: '1000m', cpu_limit: '2000m', memory_request: '4Gi', memory_limit: '8Gi', storage: '50Gi' },
  { tool_key: 'argocd', cpu_request: '250m', cpu_limit: '500m', memory_request: '512Mi', memory_limit: '1Gi', storage: '-' },
  { tool_key: 'harbor', cpu_request: '500m', cpu_limit: '1000m', memory_request: '1Gi', memory_limit: '2Gi', storage: '20Gi' },
  { tool_key: 'prometheus', cpu_request: '500m', cpu_limit: '1000m', memory_request: '2Gi', memory_limit: '4Gi', storage: '20Gi' },
]

export const NAMESPACES = ['default', 'nullus-app', 'nullus-gitlab-argocd-sonarqube', 'nullus-lite', 'kube-system']

export const COMPATIBILITY = [
  {
    id: 'gitlab-argocd-v1',
    name: 'GitLab + Argo CD',
    status: 'verified',
    kubernetes_min: '1.30',
    kubernetes_max: '1.35',
    tools: [
      { name: 'GitLab CE', helm_version: '8.7.2', app_version: '17.7.2', architectures: ['amd64', 'arm64'], min_kubernetes: '1.28', grade: 'verified' },
      { name: 'Argo CD', helm_version: '7.7.16', app_version: '2.13.2', architectures: ['amd64', 'arm64'], min_kubernetes: '1.27', grade: 'verified' },
      { name: 'Prometheus', helm_version: '67.0.0', app_version: '3.1.0', architectures: ['amd64', 'arm64'], min_kubernetes: '1.27', grade: 'verified' },
    ],
  },
]

/**
 * 스택 연동 정보. 앱 배포 화면이 저장소·레지스트리 주소의 기본값으로 쓴다.
 *
 * `component_type` 은 서버가 내는 다섯 가지 그대로여야 한다 — 핸들러가
 * code_repository · package_registry · image_registry · ci_platform · cd_tool
 * 로 만든다(stack_handler.go). 이름이 다르면 화면이 주소를 못 찾아 앞단이
 * 빈 채로 열린다. 처음에 source_repository · container_registry 로 적어
 * 캡처가 그렇게 나왔다.
 */
export const STACK_INTEGRATIONS = {
  stack_id: 'stk_7a31c0',
  state: 'completed',
  total: 5,
  integrations: [
    {
      id: 'int_stk_7a31c0_code_repository', stack_id: 'stk_7a31c0',
      component_type: 'code_repository', provider: 'GitLab CE',
      endpoint: 'https://gitlab.nullus.local', api_endpoint: 'https://gitlab.nullus.local',
      credential_ready: true, health_status: 'ready',
      provisioning_capabilities: ['repository_select', 'repository_create'],
      metadata: { version: '17.7.2' },
    },
    {
      id: 'int_stk_7a31c0_package_registry', stack_id: 'stk_7a31c0',
      component_type: 'package_registry', provider: 'GitLab Package Registry',
      endpoint: 'https://gitlab.nullus.local', api_endpoint: 'https://gitlab.nullus.local',
      credential_ready: true, health_status: 'ready',
      provisioning_capabilities: ['artifact_publish', 'sbom_publish', 'test_report_publish'],
      metadata: { version: '17.7.2' },
    },
    {
      id: 'int_stk_7a31c0_image_registry', stack_id: 'stk_7a31c0',
      component_type: 'image_registry', provider: 'Harbor',
      endpoint: 'https://harbor.nullus.local', api_endpoint: 'https://harbor.nullus.local',
      credential_ready: true, health_status: 'ready',
      provisioning_capabilities: ['image_push', 'image_pull'],
      metadata: { version: '2.11.0' },
    },
    {
      id: 'int_stk_7a31c0_ci_platform', stack_id: 'stk_7a31c0',
      component_type: 'ci_platform', provider: 'GitLab CI',
      endpoint: 'https://gitlab.nullus.local', api_endpoint: 'https://gitlab.nullus.local',
      credential_ready: true, health_status: 'ready',
      provisioning_capabilities: ['workflow_provision', 'pipeline_trigger', 'run_status_read'],
      metadata: { version: '17.7.2' },
    },
    {
      id: 'int_stk_7a31c0_cd_tool', stack_id: 'stk_7a31c0',
      component_type: 'cd_tool', provider: 'Argo CD',
      endpoint: 'https://argocd.nullus.local', api_endpoint: 'https://argocd.nullus.local',
      credential_ready: true, health_status: 'ready',
      provisioning_capabilities: ['application_provision', 'sync_trigger', 'sync_status_read'],
      metadata: { version: '2.13.2' },
    },
  ],
}

/** 스택 모니터링 스냅샷. 대시보드가 5초마다 읽는다(StackMonitoringSnapshot). */
export const STACK_MONITORING = {
  stack_id: 'stk_7a31c0',
  namespace: 'nullus-gitlab-argocd-sonarqube',
  timestamp: '2026-10-12T09:30:00Z',
  summary: {
    total_pods: 24, ready_pods: 24,
    cpu_request_millicores: 6250, cpu_limit_millicores: 11500, cpu_usage_millicores: 2840,
    memory_request_mib: 13440, memory_limit_mib: 24576, memory_usage_mib: 9216,
    storage_request_gib: 160, storage_limit_gib: 160, storage_usage_gib: 40,
    storage_usage_available: true,
    usage_available: true,
  },
  pod_status_counts: [
    { name: 'Running', count: 24 },
    { name: 'Pending', count: 0 },
    { name: 'Failed', count: 0 },
  ],
  installed_resources: [
    { kind: 'StatefulSet', name: 'gitlab-postgresql', desired_replicas: 1, ready_replicas: 1, available_replicas: 1, status: 'running' },
    { kind: 'Deployment', name: 'gitlab-webservice-default', desired_replicas: 2, ready_replicas: 2, available_replicas: 2, status: 'running' },
    { kind: 'Deployment', name: 'argocd-server', desired_replicas: 1, ready_replicas: 1, available_replicas: 1, status: 'running' },
    { kind: 'Deployment', name: 'harbor-core', desired_replicas: 1, ready_replicas: 1, available_replicas: 1, status: 'running' },
    { kind: 'StatefulSet', name: 'sonarqube', desired_replicas: 1, ready_replicas: 1, available_replicas: 1, status: 'running' },
    { kind: 'StatefulSet', name: 'prometheus-kube-prometheus-stack', desired_replicas: 1, ready_replicas: 1, available_replicas: 1, status: 'running' },
  ],
  oss_statuses: [
    {
      key: 'gitlab', name: 'GitLab CE', version: '17.7.2', enabled: true, status: 'running',
      url: 'https://gitlab.nullus.local', pod_count: 8, ready_pods: 8,
      pods: [
      { name: 'gitlab-webservice-default-7c4f8b9d4-2xk9p', phase: 'Running', ready: true, restart_count: 0, node_name: 'nullus-platform-worker', cpu_request_millicores: 500, cpu_limit_millicores: 1000, cpu_usage_millicores: 312, memory_request_mib: 2048, memory_limit_mib: 4096, memory_usage_mib: 1536, status: 'running' },
      { name: 'gitlab-sidekiq-all-in-1-v2-6d8f7c-nq4mz', phase: 'Running', ready: true, restart_count: 0, node_name: 'nullus-platform-worker', cpu_request_millicores: 400, cpu_limit_millicores: 800, cpu_usage_millicores: 186, memory_request_mib: 1536, memory_limit_mib: 3072, memory_usage_mib: 1024, status: 'running' },
      { name: 'gitlab-postgresql-0', phase: 'Running', ready: true, restart_count: 0, node_name: 'nullus-platform-worker2', cpu_request_millicores: 500, cpu_limit_millicores: 1000, cpu_usage_millicores: 240, memory_request_mib: 2048, memory_limit_mib: 4096, memory_usage_mib: 1792, status: 'running' },
      ],
    },
    {
      key: 'argocd', name: 'Argo CD', version: '2.13.2', enabled: true, status: 'running',
      url: 'https://argocd.nullus.local', pod_count: 5, ready_pods: 5,
      pods: [
      { name: 'argocd-server-5b7d9c8f4-h8vkt', phase: 'Running', ready: true, restart_count: 0, node_name: 'nullus-platform-worker', cpu_request_millicores: 100, cpu_limit_millicores: 250, cpu_usage_millicores: 42, memory_request_mib: 256, memory_limit_mib: 512, memory_usage_mib: 198, status: 'running' },
      { name: 'argocd-repo-server-69c4b8d7-wq2nl', phase: 'Running', ready: true, restart_count: 0, node_name: 'nullus-platform-worker2', cpu_request_millicores: 150, cpu_limit_millicores: 400, cpu_usage_millicores: 88, memory_request_mib: 512, memory_limit_mib: 1024, memory_usage_mib: 412, status: 'running' },
      { name: 'argocd-application-controller-0', phase: 'Running', ready: true, restart_count: 0, node_name: 'nullus-platform-worker2', cpu_request_millicores: 250, cpu_limit_millicores: 500, cpu_usage_millicores: 164, memory_request_mib: 1024, memory_limit_mib: 2048, memory_usage_mib: 876, status: 'running' },
      ],
    },
    {
      key: 'harbor', name: 'Harbor', version: '2.11.0', enabled: true, status: 'running',
      url: 'https://harbor.nullus.local', pod_count: 6, ready_pods: 6,
      pods: [
      { name: 'harbor-core-7f8d9b4c6-mp3tz', phase: 'Running', ready: true, restart_count: 0, node_name: 'nullus-platform-worker', cpu_request_millicores: 200, cpu_limit_millicores: 500, cpu_usage_millicores: 76, memory_request_mib: 512, memory_limit_mib: 1024, memory_usage_mib: 388, status: 'running' },
      { name: 'harbor-registry-5c9b7d8f4-kx7qr', phase: 'Running', ready: true, restart_count: 0, node_name: 'nullus-platform-worker2', cpu_request_millicores: 150, cpu_limit_millicores: 400, cpu_usage_millicores: 54, memory_request_mib: 512, memory_limit_mib: 1024, memory_usage_mib: 296, status: 'running' },
      ],
    },
    {
      key: 'sonarqube', name: 'SonarQube', version: '2026.5', enabled: true, status: 'warning',
      url: 'https://sonarqube.nullus.local', pod_count: 2, ready_pods: 2,
      pods: [
      { name: 'sonarqube-0', phase: 'Running', ready: true, restart_count: 1, node_name: 'nullus-platform-worker2', cpu_request_millicores: 500, cpu_limit_millicores: 2000, cpu_usage_millicores: 520, memory_request_mib: 2048, memory_limit_mib: 4096, memory_usage_mib: 2048, status: 'running' },
      ],
    },
    {
      key: 'prometheus', name: 'Prometheus', version: '3.1.0', enabled: true, status: 'running',
      url: '', pod_count: 3, ready_pods: 3,
      pods: [
      { name: 'prometheus-kube-prometheus-stack-0', phase: 'Running', ready: true, restart_count: 0, node_name: 'nullus-platform-worker', cpu_request_millicores: 250, cpu_limit_millicores: 500, cpu_usage_millicores: 182, memory_request_mib: 512, memory_limit_mib: 1024, memory_usage_mib: 486, status: 'running' },
      ],
    },
  ],
}

/** 클러스터 모니터링 요약. 클러스터 탭의 KPI 카드가 읽는다. */
export const CLUSTER_MONITORING = {
  cluster_id: 'cls_platform_0001',
  collected_at: '2026-10-12T09:30:00Z',
  pods_total: 48, pods_running: 46, pods_pending: 1, pods_failed: 1,
  cpu_usage_millicores: 3920, cpu_capacity_millicores: 16000,
  memory_usage_bytes: 12884901888, memory_capacity_bytes: 33554432000,
  pipeline_success: 18, pipeline_failed: 2,
}

/** 스택 이력. 버전마다 무엇이 바뀌었는지. */
export const STACK_HISTORY = [
  { id: 'hs_4', stack_id: 'stk_7a31c0', version: 4, action: 'add_tools', changed_by: 'admin@nullus.dev', summary: 'SonarQube 추가', created_at: '2026-10-12T04:25:00Z' },
  { id: 'hs_3', stack_id: 'stk_7a31c0', version: 3, action: 'config_update', changed_by: 'devops@nullus.dev', summary: 'Harbor 리소스 상향 (1Gi → 2Gi)', created_at: '2026-10-12T03:40:00Z' },
  { id: 'hs_2', stack_id: 'stk_7a31c0', version: 2, action: 'deploy', changed_by: 'devops@nullus.dev', summary: '초기 배포 완료', created_at: '2026-10-12T02:55:00Z' },
  { id: 'hs_1', stack_id: 'stk_7a31c0', version: 1, action: 'create', changed_by: 'devops@nullus.dev', summary: '템플릿에서 스택 생성', created_at: '2026-10-12T02:10:00Z' },
]

/** 토큰 관리 화면의 토큰 소스. */
export const TOKEN_SOURCES = [
  { id: 'ts_1', name: 'GitLab 프로젝트 토큰', kind: 'gitlab_project_token', stack_id: 'stk_7a31c0', stack_name: 'nullus-gitlab-argocd-sonarqube', status: 'active', rotation_interval: '720h', expires_at: '2026-11-11T00:00:00Z', last_rotated_at: '2026-10-12T02:50:00Z' },
  { id: 'ts_2', name: 'Harbor 로봇 계정', kind: 'harbor_robot', stack_id: 'stk_7a31c0', stack_name: 'nullus-gitlab-argocd-sonarqube', status: 'active', rotation_interval: '720h', expires_at: '2026-11-11T00:00:00Z', last_rotated_at: '2026-10-12T02:52:00Z' },
  { id: 'ts_3', name: 'Argo CD 저장소 자격증명', kind: 'argocd_repo_credential', stack_id: 'stk_7a31c0', stack_name: 'nullus-gitlab-argocd-sonarqube', status: 'rotation_due', rotation_interval: '720h', expires_at: '2026-10-14T00:00:00Z', last_rotated_at: '2026-09-14T02:52:00Z' },
]
