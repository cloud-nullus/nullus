/**
 * 둘러보기 중에만 쓰는 화면 재료.
 *
 * 투어는 처음 들어온 사람에게 흐름을 보여 주는 것이라, 정작 그 사람의 계정에는
 * 클러스터도 스택도 파이프라인도 없다. 빈 표를 강조하며 "여기서 파이프라인을
 * 봅니다" 라고 말해 봐야 아무것도 전달되지 않는다.
 *
 * 그렇다고 화면 코드에 "투어일 때는 이 값" 같은 분기를 넣지는 않는다 — 그 분기는
 * 영원히 남고 언젠가 진짜 화면에서 새어 나온다. 대신 API 경계 한 곳에서만
 * 갈아 끼운다(tour-mock-adapter.ts).
 */

const CLUSTER_ID = 'tour-cluster-1'
const STACK_ID = 'stk_tour0001'

export const TOUR_CLUSTERS = {
  items: [
    {
      id: CLUSTER_ID,
      name: 'tour-platform-cluster',
      status: 'connected',
      connection_status: 'connected',
      type: 'pipeline',
      types: ['pipeline'],
      k8s_version: '1.35',
      node_count: 3,
    },
    {
      id: 'tour-cluster-2',
      name: 'tour-target-cluster',
      status: 'connected',
      connection_status: 'connected',
      type: 'target',
      types: ['target'],
      k8s_version: '1.35',
      node_count: 2,
    },
  ],
  total: 2,
}

export const TOUR_STACKS = {
  items: [
    {
      id: STACK_ID,
      name: 'tour-lite-stack',
      status: 'running',
      state: 'completed',
      namespace: 'nullus-tour',
      cluster_id: CLUSTER_ID,
      cluster_name: 'tour-platform-cluster',
      template_id: 'gitea-jenkins-argocd-lite-v1',
      created_at: '2026-08-15T00:00:00+09:00',
      config: {
        artifacts: {
          source_repository: { name: 'Gitea', version: '1.27.0', enabled: true },
          container_registry: { name: 'Harbor', version: '2.11.0', enabled: true },
        },
        pipeline: {
          ci_platform: { name: 'Jenkins', version: '2.568.2', enabled: true },
          cd_tool: { name: 'Argo CD', version: 'v2.13.3', enabled: true },
        },
      },
    },
  ],
  total: 1,
}

export const TOUR_PIPELINES = {
  items: [
    {
      id: 'pip_tour0001',
      name: 'tour-demo-api',
      execution_mode: 'stack_integrated',
      template_id: 'web-backend-v1',
      cluster_id: CLUSTER_ID,
      stack_id: STACK_ID,
      namespace: 'tour-apps',
      app_type: 'backend',
      git_repo_url: 'http://gitea.tour.internal/nullus/tour-demo-api.git',
      status: 'active',
      created_at: '2026-08-15T00:10:00+09:00',
    },
  ],
  total: 1,
}

/** 템플릿 목록. 투어가 경량 템플릿 카드를 집어 상세를 열어 보여 준다. */
export const TOUR_TEMPLATES = [
  {
    id: 'gitea-jenkins-argocd-lite-v1',
    name: 'Gitea + Jenkins + Argo CD (Lite)',
    description:
      '8Gi 노드 하나에 올라가는 최소 구성입니다. Gitea·Jenkins·Harbor·Argo CD 만 세우고 오브젝트 스토리지와 모니터링은 뺐습니다. 로컬 검증이나 소규모 PoC 에 맞습니다.',
    tools: [
      { category: 'source_repository', name: 'Gitea', helm_version: '12.7.0', app_version: '1.27.0' },
      { category: 'ci_platform', name: 'Jenkins', helm_version: '5.9.54', app_version: '2.568.2' },
      { category: 'container_registry', name: 'Harbor', helm_version: '1.15.0', app_version: '2.11.0' },
      { category: 'cd_tool', name: 'Argo CD', helm_version: '7.7.16', app_version: 'v2.13.3' },
    ],
    estimated_install_time: 2_400_000_000_000,
    recommended_use_case: '단일 노드 로컬 검증, 소규모 PoC',
    min_resources: '4 vCPU / 8Gi RAM / 60Gi Storage',
    planning_profile: 'local',
  },
  {
    id: 'gitlab-allinone-v1',
    name: 'GitLab All-in-One',
    description: 'GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.',
    tools: [
      { category: 'source_repository', name: 'GitLab CE', helm_version: '9.5.1', app_version: '18.5.1' },
      { category: 'ci_platform', name: 'GitLab CI', helm_version: '9.5.1', app_version: '18.5.1' },
      { category: 'container_registry', name: 'GitLab Registry', helm_version: '9.5.1', app_version: '18.5.1' },
      { category: 'cd_tool', name: 'Argo CD', helm_version: '7.7.16', app_version: 'v2.13.3' },
    ],
    estimated_install_time: 5_400_000_000_000,
    recommended_use_case: '중견기업, 단일 플랫폼 선호',
    min_resources: '8 vCPU / 16Gi RAM / 100Gi Storage',
    planning_profile: 'standard',
  },
]

/** 배포된 워크로드. 투어의 "워크로드" 걸음이 설명할 대상이다. */
export const TOUR_STACK_WORKLOADS = {
  summary: { totalPipelines: 1, totalDeployments: 3, runningPods: 2, pendingPods: 0, failedPods: 0 },
  pipelines: [
    {
      id: 'pip_tour0001',
      name: 'tour-demo-api',
      namespace: 'tour-apps',
      status: 'healthy',
      lastDeployment: { id: 'dep_tour0001', status: 'succeeded', startedAt: '2026-08-15T00:20:00+09:00', version: 'v0.1.0' },
      k8sObjects: [
        { kind: 'Deployment', name: 'tour-demo-api', namespace: 'tour-apps', status: 'Available', replicas: 2, templateId: 'web-backend-v1' },
        { kind: 'Service', name: 'tour-demo-api', namespace: 'tour-apps', status: 'Active', port: 8080 },
        { kind: 'Pod', name: 'tour-demo-api-5f7c9d-abcde', namespace: 'tour-apps', status: 'Running', node: 'tour-worker-1' },
        { kind: 'Pod', name: 'tour-demo-api-5f7c9d-fghij', namespace: 'tour-apps', status: 'Running', node: 'tour-worker-2' },
      ],
    },
  ],
}

/** 스택 모니터링 스냅숏. 투어가 "배포된 모습" 을 보여 주는 걸음에 쓴다. */
export const TOUR_STACK_MONITORING = {
  stack_id: 'stk_tour0001',
  namespace: 'nullus-tour',
  timestamp: '2026-08-15T00:30:00+09:00',
  summary: {
    total_pods: 18,
    ready_pods: 18,
    cpu_request_millicores: 2400,
    cpu_limit_millicores: 4800,
    cpu_usage_millicores: 860,
    memory_request_mib: 4812,
    memory_limit_mib: 9624,
    memory_usage_mib: 2140,
    storage_request_gib: 60,
    storage_limit_gib: 120,
    storage_usage_gib: 14,
    storage_usage_available: true,
  },
  pod_status_counts: [
    { name: 'Running', count: 18 },
    { name: 'Pending', count: 0 },
    { name: 'Failed', count: 0 },
  ],
  installed_resources: [],
  oss_statuses: [
    { key: 'gitea', name: 'Gitea', version: '1.27.0', enabled: true, status: 'running', pod_count: 1, ready_pods: 1, pods: [{ name: 'gitea-6f7c9d8b4-mn2xq', phase: 'Running', ready: true, restart_count: 0, node_name: 'tour-worker-1', cpu_request_millicores: 100, cpu_limit_millicores: 200, cpu_usage_millicores: 24, memory_request_mib: 256, memory_limit_mib: 512, memory_usage_mib: 180, status: 'running' }] },
    { key: 'jenkins', name: 'Jenkins', version: '2.568.2', enabled: true, status: 'running', pod_count: 1, ready_pods: 1, pods: [{ name: 'jenkins-0', phase: 'Running', ready: true, restart_count: 0, node_name: 'tour-worker-2', cpu_request_millicores: 100, cpu_limit_millicores: 200, cpu_usage_millicores: 24, memory_request_mib: 256, memory_limit_mib: 1024, memory_usage_mib: 410, status: 'running' }] },
    { key: 'harbor', name: 'Harbor', version: '2.11.0', enabled: true, status: 'running', pod_count: 3, ready_pods: 3, pods: [{ name: 'harbor-core-7d9f8c6b5-pl4mk', phase: 'Running', ready: true, restart_count: 0, node_name: 'tour-worker-1', cpu_request_millicores: 100, cpu_limit_millicores: 200, cpu_usage_millicores: 24, memory_request_mib: 256, memory_limit_mib: 1024, memory_usage_mib: 190, status: 'running' }, { name: 'harbor-registry-58c7d94f6-qq7rt', phase: 'Running', ready: true, restart_count: 0, node_name: 'tour-worker-2', cpu_request_millicores: 100, cpu_limit_millicores: 200, cpu_usage_millicores: 24, memory_request_mib: 256, memory_limit_mib: 1024, memory_usage_mib: 120, status: 'running' }, { name: 'harbor-database-0', phase: 'Running', ready: true, restart_count: 0, node_name: 'tour-worker-1', cpu_request_millicores: 100, cpu_limit_millicores: 200, cpu_usage_millicores: 24, memory_request_mib: 256, memory_limit_mib: 1024, memory_usage_mib: 240, status: 'running' }] },
    { key: 'argocd', name: 'Argo CD', version: 'v2.13.3', enabled: true, status: 'running', pod_count: 3, ready_pods: 3, pods: [{ name: 'argo-cd-argocd-application-controller-0', phase: 'Running', ready: true, restart_count: 0, node_name: 'tour-worker-1', cpu_request_millicores: 100, cpu_limit_millicores: 200, cpu_usage_millicores: 24, memory_request_mib: 123, memory_limit_mib: 512, memory_usage_mib: 210, status: 'running' }, { name: 'argo-cd-argocd-repo-server-6b5c8d7f9-xk2ld', phase: 'Running', ready: true, restart_count: 0, node_name: 'tour-worker-2', cpu_request_millicores: 100, cpu_limit_millicores: 200, cpu_usage_millicores: 24, memory_request_mib: 102, memory_limit_mib: 512, memory_usage_mib: 150, status: 'running' }, { name: 'argo-cd-argocd-server-79d6f4b8c5-tt9wz', phase: 'Running', ready: true, restart_count: 0, node_name: 'tour-worker-1', cpu_request_millicores: 100, cpu_limit_millicores: 200, cpu_usage_millicores: 24, memory_request_mib: 102, memory_limit_mib: 512, memory_usage_mib: 130, status: 'running' }] },
  ],
}
