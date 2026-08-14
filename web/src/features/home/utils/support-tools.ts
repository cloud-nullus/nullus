// 메인 화면이 보여 주는 "Nullus 가 지원하는 OSS" 목록.
//
// 기준은 **백엔드에 실제 배선이 있는가** 다 — 설치되는 것(helm 단계 카탈로그
// `internal/stack/adapter/helm/helm_step_metadata.go` 와 차트 분기
// `resolveChartSpecForStep`)이거나, 클러스터 밖이지만 연동 구현체가 있는 것
// (`internal/cicd/adapter/github/`)이다.
//
// 설치 마법사의 선택지와 같지 않다. 마법사에는 배선이 없는 선택지가 남아 있고
// (Jenkins·Flux·Thanos 따위 — 고를 수는 있지만 배포해도 아무것도 설치되지
// 않는다), 그것까지 메인에 걸면 "지원한다" 가 거짓이 된다. 어느 쪽인지는
// support-tools.test.ts 의 NOT_INSTALLABLE 이 명시적으로 들고 있다 — 마법사에
// 선택지를 추가하면 카드로 올리든 배선 없음으로 선언하든 반드시 한쪽을 골라야
// 테스트가 통과한다.
//
// 카드는 **제품 단위**다. 같은 제품이 마법사에서 역할별로 여러 번 나오는 경우
// (GitLab = 소스·레지스트리·CI) 카드는 하나이고 wizardIds 가 그 선택지들을 모은다.

export interface SupportTool {
  /** 카드에 보이는 제품명. 브랜드명이라 번역하지 않는다. */
  name: string
  /** public/tool-icons/<icon>.svg — 각 제품의 공식 마크다. */
  icon: string
  /** 카테고리 라벨의 i18n 키. */
  categoryKey: string
  /** 이 카드가 대표하는 설치 마법사 선택지 id. */
  wizardIds: string[]
  /**
   * 흰색 단색 마크라 라이트 테마의 밝은 카드 위에서 사라진다.
   * 반전하면 정확히 검정이 되고, 그것이 이 마크의 밝은 배경용 공식 색이다.
   */
  monochromeWhite?: boolean
}

const CATEGORY = {
  source: 'home.supportTools.category.sourceRepository',
  cicd: 'home.supportTools.category.cicdPlatform',
  delivery: 'home.supportTools.category.delivery',
  registry: 'home.supportTools.category.registry',
  objectStorage: 'home.supportTools.category.objectStorage',
  metrics: 'home.supportTools.category.metrics',
  visualization: 'home.supportTools.category.visualization',
  logSearch: 'home.supportTools.category.logSearch',
  tracing: 'home.supportTools.category.tracing',
  telemetry: 'home.supportTools.category.telemetry',
  secrets: 'home.supportTools.category.secrets',
} as const

/** 순서는 DevSecOps 흐름을 따른다 — 소스 → CI → CD → 저장소 → 관측 → 비밀. */
export const SUPPORT_TOOLS: SupportTool[] = [
  { name: 'GitLab', icon: 'gitlab', categoryKey: CATEGORY.source, wizardIds: ['gitlab', 'gitlab-registry', 'gitlab-ci'] },
  // GitHub·GHCR·GitHub Actions 는 클러스터 밖이라 설치할 차트가 없다. 그래도
  // 지원 목록에 있는 것은 파이프라인을 실제로 만들어 주는 어댑터가 있기
  // 때문이다 — 리포·시크릿·워크플로 스캐폴딩까지 간다.
  { name: 'GitHub', icon: 'github', categoryKey: CATEGORY.source, wizardIds: ['github', 'ghcr'], monochromeWhite: true },
  { name: 'GitHub Actions', icon: 'githubactions', categoryKey: CATEGORY.cicd, wizardIds: ['github-actions'] },
  // Gitea 는 소스 저장소만 담당한다 — GitLab 처럼 CI 도 레지스트리도 겸하지 않아
  // wizardIds 가 하나다.
  { name: 'Gitea', icon: 'gitea', categoryKey: CATEGORY.source, wizardIds: ['gitea'] },
  { name: 'Jenkins', icon: 'jenkins', categoryKey: CATEGORY.cicd, wizardIds: ['jenkins'] },
  { name: 'Argo CD', icon: 'argo', categoryKey: CATEGORY.delivery, wizardIds: ['argocd'] },
  { name: 'Harbor', icon: 'harbor', categoryKey: CATEGORY.registry, wizardIds: ['harbor'] },
  { name: 'Nexus Repository', icon: 'sonatype', categoryKey: CATEGORY.registry, wizardIds: ['nexus'] },
  // MinIO 는 마법사가 고르게 하지 않는다 — 오브젝트 스토리지 백엔드로 고정 설치된다.
  { name: 'MinIO', icon: 'minio', categoryKey: CATEGORY.objectStorage, wizardIds: ['minio'] },
  { name: 'Prometheus', icon: 'prometheus', categoryKey: CATEGORY.metrics, wizardIds: ['prometheus'] },
  { name: 'Grafana', icon: 'grafana', categoryKey: CATEGORY.visualization, wizardIds: ['grafana'] },
  // OpenSearch Dashboards 는 여기 없다 — 마법사의 시각화 선택지로 남아 있지만
  // 설치하는 단계가 없다. OpenSearch 본체만 로그 검색 저장소로 설치된다.
  { name: 'OpenSearch', icon: 'opensearch', categoryKey: CATEGORY.logSearch, wizardIds: ['opensearch'] },
  // Loki·Tempo 는 Grafana Labs 제품이고 별도 마크가 배포되지 않아 같은 마크를 쓴다.
  // 이름이 다르므로 카드가 구분된다 — 지원 목록에서 빼는 것보다 낫다.
  { name: 'Grafana Loki', icon: 'grafana', categoryKey: CATEGORY.logSearch, wizardIds: ['loki'] },
  { name: 'Grafana Tempo', icon: 'grafana', categoryKey: CATEGORY.tracing, wizardIds: ['tempo'] },
  { name: 'Jaeger', icon: 'jaeger', categoryKey: CATEGORY.tracing, wizardIds: ['jaeger'] },
  { name: 'OpenTelemetry Collector', icon: 'opentelemetry', categoryKey: CATEGORY.telemetry, wizardIds: ['opentelemetry-collector'] },
  { name: 'OpenBao', icon: 'openbao', categoryKey: CATEGORY.secrets, wizardIds: ['openbao'] },
]

/**
 * 공식 마크의 경로.
 *
 * stack 의 toolLogoURL 과 달리 이름을 추측하지 않는다 — 그쪽은 못 찾으면
 * 쿠버네티스 마크로 떨어지는데, 여기서는 엉뚱한 제품 로고가 조용히 붙는다.
 */
export function supportToolIconURL(icon: string): string {
  return `/tool-icons/${icon}.svg`
}
