export function toolLogoURL(toolName: string): string {
  const key = toolName
    .toLowerCase()
    .replace(/[_-]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();
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
    seaweedfs: "seaweedfs",
    openbao: "openbao",
  };
  const slug = map[key] ?? "kubernetes";
  return `/tool-icons/${slug}.svg`;
}
