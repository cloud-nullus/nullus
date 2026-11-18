function toolKey(toolName: string): string {
  return toolName
    .toLowerCase()
    .replace(/[_-]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

// minio 슬롯을 고르면 서는 것은 MinIO 서버의 커뮤니티 포크 SILO 다
// (shareddomain.MinIOServerImage). 화면은 이것을 SILO 로만 적는다 — MinIO 상표 정책이
// 수정판에 MinIO 이름을 붙이거나 다른 단어와 묶는 것("MinIO (SILO)")을 금지하고, 대신
// "based on MinIO technology" 같은 설명을 권한다. 설명 문구는 i18n 의 minio.description.
export const MINIO_DISPLAY_NAME = "SILO";

const DISPLAY_NAME_BY_KEY: Record<string, string> = {
  minio: MINIO_DISPLAY_NAME,
  silo: MINIO_DISPLAY_NAME,
  "minio (silo)": MINIO_DISPLAY_NAME,
  seaweedfs: "SeaweedFS",
};

/**
 * 화면에 쓰는 도구 이름.
 *
 * 스택 설정·서버 응답의 이름은 저장된 그대로(마법사로 만든 스택은 id, 템플릿은 카탈로그
 * 이름)라 대부분 그대로 둔다. 오브젝트 스토리지만 고친다 — 구현체 id(minio·seaweedfs)가
 * 그대로 보이고, minio 로 설치되는 것은 다른 제품(SILO)이다. 이름으로 무언가를
 * 찾는 곳(접속 주소·자격 조회)에는 원래 이름을 넘긴다.
 */
export function toolDisplayName(toolName: string): string {
  return DISPLAY_NAME_BY_KEY[toolKey(toolName)] ?? toolName;
}

/**
 * 도구를 가리키는 두 글자 약자. 화면은 OSS 를 이 약자로 표시한다.
 *
 * 화면은 OSS 를 **브랜드 로고로 표시하지 않는다.** 코드 라이선스(Apache·AGPL 등)는
 * 상표 사용권을 주지 않고(Apache-2.0 §6), 로고는 각 상표권자의 정책을 따른다. 그 정책이
 * 제품 화면·웹사이트·홍보물의 로고 사용을 서면 허락 대상으로 두거나(Linux Foundation,
 * Grafana Labs) 아예 거절한다고 적어 두었다(GitLab: 제품 UI 의 로고 요청은 "generally
 * refused"). 이름을 글자로 적어 "무엇을 설치·연동한다" 고 밝히는 것은 사실 표시라
 * 허용된다. 그래서 모든 도구가 같은 규칙을 따른다 — 이름은 글자로, 표식은 같은 모양의
 * 타일에 든 약자로(ToolMark). 제품 고유의 글꼴·색을 흉내 내지 않는다.
 *
 * 약자는 제품 단위다. 한 제품이 역할별 이름으로 와도(gitlab·gitlab-ci·gitlab-registry)
 * 같은 약자이고, 메인 화면의 카드끼리는 겹치지 않는다(support-tools.test.ts).
 * 표에 없는 이름은 규칙으로 만든다. 새 도구를 붙일 때 그 제품의 로고 파일을 public/ 에
 * 다시 두지 않는다(메인 화면 테스트가 막는다).
 */
const MONOGRAM_BY_KEY: Record<string, string> = {
  gitlab: "GL",
  "gitlab ce": "GL",
  "gitlab ci": "GL",
  "gitlab runner": "GL",
  "gitlab registry": "GL",
  "gitlab package registry": "GL",
  github: "GH",
  ghcr: "GH",
  "github packages": "GH",
  "github actions": "GA",
  gitea: "GT",
  jenkins: "JE",
  tekton: "TK",

  argocd: "AR",
  "argo cd": "AR",
  flux: "FX",
  "flux cd": "FX",
  fluxcd: "FX",
  spinnaker: "SP",

  harbor: "HB",
  "harbor registry": "HB",
  nexus: "NX",
  nexus3: "NX",
  "nexus repository": "NX",
  "nexus repository manager": "NX",
  jfrog: "JF",
  "jfrog artifactory": "JF",
  docker: "DK",
  "docker hub": "DK",
  "docker registry": "DK",

  // 백엔드는 이 슬롯을 계속 minio 로 부르지만 설치되는 실물은 SILO 다. 예전 화면이
  // 저장한 "MinIO (SILO)" 도 받는다.
  minio: "SI",
  silo: "SI",
  "minio (silo)": "SI",
  seaweedfs: "SW",
  gcs: "GC",
  "google cloud storage": "GC",
  s3: "S3",
  "azure blob": "AZ",

  prometheus: "PR",
  thanos: "TH",
  victoriametrics: "VM",
  "victoria metrics": "VM",
  "metrics server": "MS",

  grafana: "GF",
  "opensearch dashboards": "OD",
  loki: "LK",
  "grafana loki": "LK",
  opensearch: "OS",
  elasticsearch: "ES",
  fluentd: "FD",
  tempo: "TP",
  "grafana tempo": "TP",
  jaeger: "JG",
  opentelemetry: "OT",
  "opentelemetry collector": "OT",

  openbao: "OB",
  "external secrets": "XS",
  trivy: "TV",
  sonarqube: "SQ",
  postgresql: "PG",
  postgres: "PG",
  mysql: "MY",
  mariadb: "MA",
  keycloak: "KC",
  "cert manager": "CM",
};

/**
 * 도구 이름 → 두 글자 약자. 표에 없으면 첫 단어의 앞 두 글자다.
 *
 * 단어마다 첫 글자를 따는 규칙은 쓰지 않는다 — "GitLab + Argo CD" 같은 묶음 이름이
 * GitHub Actions 와 같은 GA 가 됐다.
 */
export function toolMonogram(toolName: string): string {
  const key = toolKey(toolName);
  const known = MONOGRAM_BY_KEY[key];
  if (known) return known;
  const first = key.split(/[^a-z0-9]+/).find(Boolean);
  return first ? first.slice(0, 2).toUpperCase() : "?";
}
