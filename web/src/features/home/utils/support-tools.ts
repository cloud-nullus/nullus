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

import { MINIO_DISPLAY_NAME } from '../../stack/utils/tool-identity'

/** 카드의 카테고리 — 값이 곧 i18n 키 `home.supportTools.category.<값>` 의 끝이다. */
export type SupportToolCategory =
  | 'sourceRepository'
  | 'cicdPlatform'
  | 'delivery'
  | 'registry'
  | 'objectStorage'
  | 'metrics'
  | 'visualization'
  | 'logSearch'
  | 'tracing'
  | 'telemetry'
  | 'secrets'

export interface SupportTool {
  /** 카드에 보이는 제품명. 브랜드명이라 번역하지 않는다. */
  name: string
  /** 카드 아래 줄의 카테고리 라벨. */
  category: SupportToolCategory
  /** 이 카드가 대표하는 설치 마법사 선택지 id. */
  wizardIds: string[]
}

/** 카테고리 라벨의 i18n 키. */
export function supportToolCategoryKey(category: SupportToolCategory): string {
  return `home.supportTools.category.${category}`
}

/** 순서는 DevSecOps 흐름을 따른다 — 소스 → CI → CD → 저장소 → 관측 → 비밀. */
export const SUPPORT_TOOLS: SupportTool[] = [
  { name: 'GitLab', category: 'sourceRepository', wizardIds: ['gitlab', 'gitlab-registry', 'gitlab-ci'] },
  // GitHub·GHCR·GitHub Actions 는 클러스터 밖이라 설치할 차트가 없다. 그래도
  // 지원 목록에 있는 것은 파이프라인을 실제로 만들어 주는 어댑터가 있기
  // 때문이다 — 리포·시크릿·워크플로 스캐폴딩까지 간다.
  { name: 'GitHub', category: 'sourceRepository', wizardIds: ['github', 'ghcr'] },
  { name: 'GitHub Actions', category: 'cicdPlatform', wizardIds: ['github-actions'] },
  // Gitea 는 소스 저장소만 담당한다 — GitLab 처럼 CI 도 레지스트리도 겸하지 않아
  // wizardIds 가 하나다.
  { name: 'Gitea', category: 'sourceRepository', wizardIds: ['gitea'] },
  { name: 'Jenkins', category: 'cicdPlatform', wizardIds: ['jenkins'] },
  { name: 'Argo CD', category: 'delivery', wizardIds: ['argocd'] },
  { name: 'Harbor', category: 'registry', wizardIds: ['harbor'] },
  { name: 'Nexus Repository', category: 'registry', wizardIds: ['nexus'] },
  // 오브젝트 스토리지는 마법사의 Storage 탭에서 구현체를 고른다(S3 계약 뒤에서 갈아 끼운다).
  // MinIO 로 설치되는 실물은 MinIO 포크인 SILO 다(받을 수 있는 이미지가 그쪽뿐이다 —
  // shareddomain.MinIOServerImage). 이름도 설치되는 것과 같아야 한다.
  { name: MINIO_DISPLAY_NAME, category: 'objectStorage', wizardIds: ['minio'] },
  { name: 'SeaweedFS', category: 'objectStorage', wizardIds: ['seaweedfs'] },
  { name: 'Prometheus', category: 'metrics', wizardIds: ['prometheus'] },
  { name: 'Grafana', category: 'visualization', wizardIds: ['grafana'] },
  // OpenSearch Dashboards 는 여기 없다 — 마법사의 시각화 선택지로 남아 있지만
  // 설치하는 단계가 없다. OpenSearch 본체만 로그 검색 저장소로 설치된다.
  { name: 'OpenSearch', category: 'logSearch', wizardIds: ['opensearch'] },
  { name: 'Grafana Loki', category: 'logSearch', wizardIds: ['loki'] },
  { name: 'Grafana Tempo', category: 'tracing', wizardIds: ['tempo'] },
  { name: 'Jaeger', category: 'tracing', wizardIds: ['jaeger'] },
  { name: 'OpenTelemetry Collector', category: 'telemetry', wizardIds: ['opentelemetry-collector'] },
  { name: 'OpenBao', category: 'secrets', wizardIds: ['openbao'] },
]
