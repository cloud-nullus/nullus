function toolKey(toolName: string): string {
  return toolName
    .toLowerCase()
    .replace(/[_-]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

// minio 를 고르면 서는 것은 MinIO 포크 SILO 다(shareddomain.MinIOServerImage). 마크가 SILO
// 이므로 이름도 둘을 함께 적는다 — "MinIO" 만 적으면 마크와 이름이 다른 제품을 가리킨다.
export const MINIO_DISPLAY_NAME = "MinIO (SILO)";

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
 * 그대로 보이고, minio 는 마크(silo.svg)와 다른 제품 이름이 된다. 이름으로 무언가를
 * 찾는 곳(접속 주소·자격 조회)에는 원래 이름을 넘긴다.
 */
export function toolDisplayName(toolName: string): string {
  return DISPLAY_NAME_BY_KEY[toolKey(toolName)] ?? toolName;
}

export function toolLogoURL(toolName: string): string {
  const key = toolKey(toolName);
  const map: Record<string, string> = {
    gitlab: "gitlab",
    "gitlab ce": "gitlab",
    "gitlab ci": "gitlab",
    "gitlab registry": "gitlab",
    "gitlab package registry": "gitlab",
    github: "github",
    "github actions": "githubactions",
    "github packages": "github",
    nexus: "sonatype",
    "nexus repository": "sonatype",
    "nexus repository manager": "sonatype",
    jfrog: "jfrog",
    "jfrog artifactory": "jfrog",
    gitea: "gitea",
    docker: "docker",
    "docker hub": "docker",
    "docker registry": "docker",
    gcs: "googlecloud",
    "google cloud storage": "googlecloud",
    argocd: "argo",
    "argo cd": "argo",
    flux: "flux",
    "flux cd": "flux",
    fluxcd: "flux",
    jenkins: "jenkins",
    spinnaker: "spinnaker",
    tekton: "tekton",
    grafana: "grafana",
    prometheus: "prometheus",
    thanos: "thanos",
    victoriametrics: "victoriametrics",
    "victoria metrics": "victoriametrics",
    loki: "grafana",
    "grafana loki": "grafana",
    opensearch: "opensearch",
    "opensearch dashboards": "opensearch",
    elasticsearch: "elasticsearch",
    "opentelemetry collector": "opentelemetry",
    tempo: "grafana",
    jaeger: "jaeger",
    fluentd: "fluentd",
    harbor: "harbor",
    "harbor registry": "harbor",
    // 백엔드는 이 슬롯을 계속 minio 로 부른다(호환성 매트릭스·시드). 설치되는
    // 실물은 MinIO 포크인 SILO 라 마크는 그쪽을 쓴다.
    minio: "silo",
    silo: "silo",
    "minio (silo)": "silo",
    seaweedfs: "seaweedfs",
    openbao: "openbao",
  };
  const slug = map[key] ?? "kubernetes";
  return `/tool-icons/${slug}.svg`;
}
