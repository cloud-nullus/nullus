import YAML from 'yaml'
import type { StackConfigDraft } from '../stores/stack-config-store'
import { getToolAppVersion, getToolChartVersion } from '../stores/stack-config-store'
import {
  TOOL_HELM_META,
  TOOL_INSTALL_METHOD,
  TOOL_LABEL_MAP,
  TOOL_DEFAULT_IMAGE_REPOSITORY,
  GATEWAY_MANIFEST_ID,
  getManifestBundleId,
} from './install-constants'
import type { ManifestInstallType, ResourceVector } from './install-planning-utils'
import { round2 } from './install-planning-utils'

export type ManifestToolEntry = {
  toolId: string
  toolLabel: string
  installType: ManifestInstallType
  toolVersion: string
  chartVersion?: string
  hasVersionConflict: boolean
  roles: string[]
  sourceToolIds: string[]
  sourceVersions: string[]
}

export type ToolManifestResourceSpec = {
  requests: { cpu: number; memory: string; storage: string }
  limits: { cpu: number; memory: string; storage: string }
}

export function normalizeAccessDomain(domain: string): string {
  return domain.trim().replace(/\.intenral$/i, '.internal')
}

/**
 * 스택 기본 네임스페이스. 서버의 domain.DefaultStackNamespaceFor 와 같은 규칙이다.
 *
 * 예전 기본값은 'nullus' 였고 그것은 플랫폼 자신이 사는 네임스페이스였다.
 * 그래서 설치는 플랫폼의 nullus-postgresql 과 이름이 충돌해 실패했고, 삭제는
 * 플랫폼 리소스를 지웠다 — 2026-08-20 에 nullus.io 가 통째로 내려갔다.
 */
export function defaultStackNamespace(stackName: string): string {
  const slug = stackName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  if (!slug) {
    return "nullus-stack";
  }
  return `nullus-${slug}`.slice(0, 63).replace(/-+$/, "");
}

export function buildDefaultStackName(now = new Date()): string {
  const pad = (value: number) => value.toString().padStart(2, '0')
  const year = now.getFullYear()
  const month = pad(now.getMonth() + 1)
  const day = pad(now.getDate())
  const hour = pad(now.getHours())
  const minute = pad(now.getMinutes())
  const second = pad(now.getSeconds())
  return `nullus-devsecops-stack-${year}${month}${day}-${hour}${minute}${second}`
}

export function buildOpenSearchBackendTLSPolicy(namespace: string, stackName: string, accessDomain: string): Record<string, unknown> {
  const serviceName = 'opensearch-cluster-master'
  const serviceHost = `${serviceName}.${namespace}.svc.cluster.local`
  const routeHost = `opensearch.${accessDomain}`

  return {
    apiVersion: 'gateway.networking.k8s.io/v1',
    kind: 'BackendTLSPolicy',
    metadata: {
      name: 'opensearch-backend-tls',
      namespace,
      labels: {
        'nullus.io/stack-name': stackName,
        'nullus.io/type': 'gateway-backend-tls',
      },
    },
    spec: {
      targetRefs: [
        {
          group: '',
          kind: 'Service',
          name: serviceName,
        },
      ],
      validation: {
        hostname: serviceHost,
        subjectAltNames: [
          { type: 'Hostname', value: serviceHost },
          { type: 'Hostname', value: serviceName },
          { type: 'Hostname', value: routeHost },
        ],
        wellKnownCACertificates: 'System',
      },
    },
  }
}

export function toolLabel(toolId: string, noneLabel = 'Not selected'): string {
  if (!toolId) {
    return noneLabel
  }
  return TOOL_LABEL_MAP.get(toolId) ?? toolId
}

export function getHelmMeta(toolId: string) {
  return TOOL_HELM_META[toolId] ?? { repoUrl: 'https://charts.example.com', chartName: `nullus/${toolId}` }
}

export function getInstallType(toolId: string): ManifestInstallType {
  if (TOOL_INSTALL_METHOD[toolId]) return TOOL_INSTALL_METHOD[toolId]
  return TOOL_HELM_META[toolId] ? 'helm' : 'yaml'
}

/**
 * 도구 호스트가 가리켜야 할 클러스터 안 서비스.
 *
 * 서버(internal/stack/domain/gateway_backends.go)와 같은 값이어야 한다. 예전에는
 * 여기 없는 도구를 "<도구>-svc:80" 으로 지어냈는데, 실제 이름과 포트는 도구마다
 * 다르다(gitea-http:3000, jenkins:8080, nexus:8081). 그래서 UI 로 설치하면
 * gitea·harbor·jenkins·nexus 라우트가 존재하지 않는 서비스를 가리켰고, 설치는
 * 성공했는데 그 주소만 열리지 않았다.
 *
 * 표 형태로 두는 이유가 있다 — 서버 테스트가 이 파일을 읽어 값이 갈라지지 않았는지
 * 검사한다(TestGatewayBackends_MatchInstallWizard).
 */
export const GATEWAY_BACKENDS: Record<string, { serviceName: string; port: number }> = {
  gitlab: { serviceName: 'gitlab-webservice-default', port: 8181 },
  argocd: { serviceName: 'argo-cd-argocd-server', port: 80 },
  gitea: { serviceName: 'gitea-http', port: 3000 },
  jenkins: { serviceName: 'jenkins', port: 8080 },
  harbor: { serviceName: 'harbor', port: 80 },
  nexus: { serviceName: 'nexus', port: 8081 },
  minio: { serviceName: 'nullus-minio-console', port: 9001 },
  grafana: { serviceName: 'grafana', port: 80 },
  prometheus: { serviceName: 'kube-prometheus-stack-prometheus', port: 9090 },
  opensearch: { serviceName: 'opensearch-cluster-master', port: 9200 },
  openbao: { serviceName: 'openbao', port: 8200 },
}

export function gatewayBackendForTool(toolId: string): { serviceName: string; port: number } {
  const key = toolId.trim().toLowerCase().replace(/[_\s]+/g, '-')
  return (
    GATEWAY_BACKENDS[key] ??
    GATEWAY_BACKENDS[key.replace(/^argo-cd$/, 'argocd')] ??
    // 모르는 도구는 예전 규칙을 남긴다. 서버가 받아서 바로잡을 기회가 있고,
    // 여기서 던지면 매니페스트 미리보기 자체가 깨진다.
    { serviceName: `${toolId}-svc`, port: 80 }
  )
}

export function workloadContainerPortForTool(toolId: string): number {
  switch (toolId) {
    case 'grafana':
      return 3000
    case 'prometheus':
      return 9090
    case 'loki':
      return 3100
    default:
      return 8080
  }
}

export function resolveToolImage(toolId: string, toolVersion: string): string {
  const repository = TOOL_DEFAULT_IMAGE_REPOSITORY[toolId] ?? `ghcr.io/cloud-nullus/${toolId}`
  const version = toolVersion || getToolAppVersion(toolId)
  return `${repository}:${version}`
}

export function buildToolManifest(
  toolId: string,
  toolLabelValue: string,
  draft: StackConfigDraft,
  resources: ResourceVector,
  toolVersion: string,
  chartVersion?: string
): string {
  const installType = getInstallType(toolId)
  const helmMeta = getHelmMeta(toolId)
  const namespace = draft.namespace.trim() || 'nullus'
  const resourcesSpec: ToolManifestResourceSpec = {
    requests: {
      cpu: resources.cpuRequest,
      memory: `${resources.memoryRequestGi.toFixed(2)}Gi`,
      storage: `${resources.storageRequestGi.toFixed(2)}Gi`,
    },
    limits: {
      cpu: resources.cpuLimit,
      memory: `${resources.memoryLimitGi.toFixed(2)}Gi`,
      storage: `${resources.storageLimitGi.toFixed(2)}Gi`,
    },
  }

  if (installType === 'helm') {
    const valuesYaml = {
      global: {
        stackName: draft.stackName,
        accessDomain: draft.accessDomain || `${draft.stackName}.internal`,
        clusterId: draft.clusterId ?? '',
        namespace,
        toolId,
        toolLabel: toolLabelValue,
      },
      chart: {
        repoUrl: helmMeta.repoUrl,
        name: helmMeta.chartName,
        version: chartVersion || getToolChartVersion(toolId) || toolVersion,
      },
      image: {
        tag: toolVersion || getToolAppVersion(toolId),
      },
      resources: resourcesSpec,
      storage: {
        planMode: draft.storage.planMode,
        database: {
          mode: draft.storage.database.mode,
          existingRef: draft.storage.database.existingRef,
          endpoint: draft.storage.database.endpoint,
          resourceName: draft.storage.database.resourceName,
          accessSecretRef: draft.storage.database.accessSecretRef,
          authId: draft.storage.database.authId,
          authPasswordKey: draft.storage.database.authPasswordKey,
          providerOrEngine: draft.storage.database.providerOrEngine,
          version: draft.storage.database.version,
          size: draft.storage.database.size,
        },
        objectStorage: {
          mode: draft.storage.objectStorage.mode,
          existingRef: draft.storage.objectStorage.existingRef,
          endpoint: draft.storage.objectStorage.endpoint,
          resourceName: draft.storage.objectStorage.resourceName,
          accessSecretRef: draft.storage.objectStorage.accessSecretRef,
          authId: draft.storage.objectStorage.authId,
          authPasswordKey: draft.storage.objectStorage.authPasswordKey,
          providerOrEngine: draft.storage.objectStorage.providerOrEngine,
          version: draft.storage.objectStorage.version,
          size: draft.storage.objectStorage.size,
        },
      },
    }

    return YAML.stringify(valuesYaml, { indent: 2, lineWidth: 0 })
  }

  if (toolId === 'tempo') {
    const configMap = {
      apiVersion: 'v1',
      kind: 'ConfigMap',
      metadata: {
        name: 'tempo-config',
        namespace,
        labels: {
          app: toolId,
          'nullus.io/stack-name': draft.stackName,
          'nullus.io/tool-id': toolId,
        },
      },
      data: {
        'tempo.yaml': [
          'server:',
          '  http_listen_port: 3200',
          'distributor:',
          '  receivers:',
          '    otlp:',
          '      protocols:',
          '        grpc:',
          '          endpoint: 0.0.0.0:4317',
          '        http:',
          '          endpoint: 0.0.0.0:4318',
          'ingester:',
          '  trace_idle_period: 10s',
          '  max_block_duration: 5m',
          'compactor:',
          '  compaction:',
          '    block_retention: 24h',
          'storage:',
          '  trace:',
          '    backend: local',
          '    local:',
          '      path: /var/tempo/traces',
          '    wal:',
          '      path: /var/tempo/wal',
        ].join('\n'),
      },
    }

    const deployment = {
      apiVersion: 'apps/v1',
      kind: 'Deployment',
      metadata: {
        name: toolId,
        namespace,
        labels: {
          app: toolId,
          'nullus.io/stack-name': draft.stackName,
          'nullus.io/cluster-id': draft.clusterId ?? '',
          'nullus.io/tool-id': toolId,
        },
      },
      spec: {
        replicas: 1,
        selector: { matchLabels: { app: toolId } },
        template: {
          metadata: { labels: { app: toolId } },
          spec: {
            containers: [
              {
                name: toolId,
                image: resolveToolImage(toolId, toolVersion),
                args: ['-config.file=/etc/tempo/tempo.yaml'],
                ports: [
                  { name: 'http', containerPort: 3200 },
                  { name: 'otlp-grpc', containerPort: 4317 },
                  { name: 'otlp-http', containerPort: 4318 },
                ],
                volumeMounts: [
                  { name: 'tempo-config', mountPath: '/etc/tempo' },
                  { name: 'tempo-data', mountPath: '/var/tempo' },
                ],
                resources: {
                  requests: {
                    cpu: String(resources.cpuRequest),
                    memory: resourcesSpec.requests.memory,
                  },
                  limits: {
                    cpu: String(resources.cpuLimit),
                    memory: resourcesSpec.limits.memory,
                  },
                },
              },
            ],
            volumes: [
              { name: 'tempo-config', configMap: { name: 'tempo-config' } },
              { name: 'tempo-data', emptyDir: {} },
            ],
          },
        },
      },
    }

    const service = {
      apiVersion: 'v1',
      kind: 'Service',
      metadata: {
        name: 'tempo-svc',
        namespace,
        labels: { app: toolId },
      },
      spec: {
        selector: { app: toolId },
        ports: [
          { name: 'http', port: 3200, targetPort: 3200 },
          { name: 'otlp-grpc', port: 4317, targetPort: 4317 },
          { name: 'otlp-http', port: 4318, targetPort: 4318 },
        ],
      },
    }

    return [
      YAML.stringify(configMap, { indent: 2, lineWidth: 0 }),
      YAML.stringify(deployment, { indent: 2, lineWidth: 0 }),
      YAML.stringify(service, { indent: 2, lineWidth: 0 }),
    ].join('\n---\n')
  }

  const deployment = {
    apiVersion: 'apps/v1',
    kind: 'Deployment',
    metadata: {
      name: toolId,
      namespace,
      labels: {
        app: toolId,
        'nullus.io/stack-name': draft.stackName,
        'nullus.io/cluster-id': draft.clusterId ?? '',
        'nullus.io/tool-id': toolId,
      },
    },
    spec: {
      replicas: 1,
      selector: { matchLabels: { app: toolId } },
      template: {
        metadata: { labels: { app: toolId } },
        spec: {
          containers: [
            {
              name: toolId,
              image: resolveToolImage(toolId, toolVersion),
              ports: [{ name: 'http', containerPort: workloadContainerPortForTool(toolId) }],
              resources: {
                requests: {
                  cpu: String(resources.cpuRequest),
                  memory: resourcesSpec.requests.memory,
                },
                limits: {
                  cpu: String(resources.cpuLimit),
                  memory: resourcesSpec.limits.memory,
                },
              },
            },
          ],
        },
      },
    },
  }

  const service = {
    apiVersion: 'v1',
    kind: 'Service',
    metadata: {
      name: `${toolId}-svc`,
      namespace,
      labels: { app: toolId },
    },
    spec: {
      selector: { app: toolId },
      ports: [{ name: 'http', port: 80, targetPort: workloadContainerPortForTool(toolId) }],
    },
  }

  return [YAML.stringify(deployment, { indent: 2, lineWidth: 0 }), YAML.stringify(service, { indent: 2, lineWidth: 0 })]
    .join('\n---\n')
}

export function cpuQuantityFromCores(cores: number): string {
  if (!Number.isFinite(cores) || cores <= 0) return '0'
  const milli = Math.round(cores * 1000)
  if (milli <= 0) return '0'
  if (milli % 1000 === 0) {
    return `${milli / 1000}`
  }
  return `${milli}m`
}

export function giQuantity(gi: number): string {
  if (!Number.isFinite(gi) || gi <= 0) return '0Gi'
  if (Math.abs(gi - Math.round(gi)) < 1e-9) {
    return `${Math.round(gi)}Gi`
  }
  return `${gi.toFixed(2)}Gi`
}

export function toK8sResources(resources: ResourceVector): Record<string, unknown> {
  return {
    requests: {
      cpu: cpuQuantityFromCores(resources.cpuRequest),
      memory: giQuantity(resources.memoryRequestGi),
    },
    limits: {
      cpu: cpuQuantityFromCores(resources.cpuLimit),
      memory: giQuantity(resources.memoryLimitGi),
    },
  }
}

export function scaleResourceVector(resources: ResourceVector, ratio: number): ResourceVector {
  const safeRatio = Number.isFinite(ratio) && ratio > 0 ? ratio : 1
  return {
    cpuRequest: round2(Math.max(0.05, resources.cpuRequest * safeRatio)),
    cpuLimit: round2(Math.max(0.1, resources.cpuLimit * safeRatio)),
    memoryRequestGi: round2(Math.max(0.08, resources.memoryRequestGi * safeRatio)),
    memoryLimitGi: round2(Math.max(0.16, resources.memoryLimitGi * safeRatio)),
    storageRequestGi: round2(Math.max(0, resources.storageRequestGi * safeRatio)),
    storageLimitGi: round2(Math.max(0, resources.storageLimitGi * safeRatio)),
  }
}

export function buildHelmStepResourceOverride(toolId: string, resources: ResourceVector): { key: string; values: Record<string, unknown> } | null {
  const k8sResources = toK8sResources(resources)
  switch (getManifestBundleId(toolId)) {
    case 'cert-manager':
      return { key: 'installing_cert_manager', values: { resources: k8sResources } }
    case 'minio':
      return { key: 'installing_minio', values: { resources: k8sResources } }
    case 'gitlab':
      const gitlabWebVector = scaleResourceVector(resources, 0.22)
      gitlabWebVector.cpuRequest = Math.max(gitlabWebVector.cpuRequest, 0.4)
      gitlabWebVector.cpuLimit = Math.max(gitlabWebVector.cpuLimit, 0.8)
      gitlabWebVector.memoryRequestGi = Math.max(gitlabWebVector.memoryRequestGi, 1)
      gitlabWebVector.memoryLimitGi = Math.max(gitlabWebVector.memoryLimitGi, 2)
      const gitlabWeb = toK8sResources(gitlabWebVector)

      const gitlabSidekiqVector = scaleResourceVector(resources, 0.18)
      gitlabSidekiqVector.cpuRequest = Math.max(gitlabSidekiqVector.cpuRequest, 0.35)
      gitlabSidekiqVector.cpuLimit = Math.max(gitlabSidekiqVector.cpuLimit, 0.7)
      gitlabSidekiqVector.memoryRequestGi = Math.max(gitlabSidekiqVector.memoryRequestGi, 1)
      gitlabSidekiqVector.memoryLimitGi = Math.max(gitlabSidekiqVector.memoryLimitGi, 2)
      const gitlabSidekiq = toK8sResources(gitlabSidekiqVector)
      const gitlabToolboxVector = scaleResourceVector(resources, 0.08)
      gitlabToolboxVector.cpuRequest = Math.max(gitlabToolboxVector.cpuRequest, 0.25)
      gitlabToolboxVector.cpuLimit = Math.max(gitlabToolboxVector.cpuLimit, 0.5)
      gitlabToolboxVector.memoryRequestGi = Math.max(gitlabToolboxVector.memoryRequestGi, 1)
      gitlabToolboxVector.memoryLimitGi = Math.max(gitlabToolboxVector.memoryLimitGi, 2)
      const gitlabToolbox = toK8sResources(gitlabToolboxVector)
      const gitlabGitaly = toK8sResources(scaleResourceVector(resources, 0.2))
      const gitlabKas = toK8sResources(scaleResourceVector(resources, 0.12))
      const gitlabExporter = toK8sResources(scaleResourceVector(resources, 0.05))
      const gitlabRegistry = toK8sResources(scaleResourceVector(resources, 0.12))
      const gitlabRedis = toK8sResources(scaleResourceVector(resources, 0.12))
      const gitlabProm = toK8sResources(scaleResourceVector(resources, 0.08))
      return {
        key: 'installing_gitlab',
        values: {
          gitlab: {
            webservice: { resources: gitlabWeb },
            sidekiq: { resources: gitlabSidekiq },
            toolbox: { resources: gitlabToolbox },
            gitaly: { resources: gitlabGitaly },
            kas: { resources: gitlabKas },
            'gitlab-exporter': { resources: gitlabExporter },
          },
          registry: { resources: gitlabRegistry },
          redis: { master: { resources: gitlabRedis } },
          prometheus: { server: { resources: gitlabProm } },
        },
      }
    case 'argocd':
      const argoController = toK8sResources(scaleResourceVector(resources, 0.24))
      const argoRepo = toK8sResources(scaleResourceVector(resources, 0.2))
      const argoServer = toK8sResources(scaleResourceVector(resources, 0.2))
      const argoRedis = toK8sResources(scaleResourceVector(resources, 0.12))
      const argoDex = toK8sResources(scaleResourceVector(resources, 0.1))
      const argoAppSet = toK8sResources(scaleResourceVector(resources, 0.07))
      const argoNotifications = toK8sResources(scaleResourceVector(resources, 0.07))
      return {
        key: 'installing_argocd',
        values: {
          controller: { resources: argoController },
          repoServer: { resources: argoRepo },
          server: { resources: argoServer },
          redis: { resources: argoRedis },
          dex: { resources: argoDex },
          applicationSet: { resources: argoAppSet },
          notifications: { resources: argoNotifications },
        },
      }
    case 'gitlab-runner':
      return { key: 'installing_runner', values: { resources: k8sResources } }
    case 'prometheus':
      return {
        key: 'installing_prometheus',
        values: {
          prometheus: { prometheusSpec: { resources: k8sResources } },
          alertmanager: { alertmanagerSpec: { resources: k8sResources } },
          'kube-state-metrics': { resources: k8sResources },
          prometheusOperator: { resources: k8sResources },
          'prometheus-node-exporter': { resources: k8sResources },
        },
      }
    case 'grafana':
      return { key: 'installing_grafana', values: { resources: k8sResources } }
    case 'loki':
      return {
        key: 'installing_logging',
        values: {
          resources: k8sResources,
          loki: { resources: k8sResources },
          singleBinary: { resources: k8sResources },
          read: { resources: k8sResources },
          write: { resources: k8sResources },
          backend: { resources: k8sResources },
          promtail: { resources: k8sResources },
        },
      }
    case 'opensearch':
    case 'elasticsearch':
      return {
        key: 'installing_log_search',
        values: {
          resources: k8sResources,
          master: { resources: k8sResources },
        },
      }
    case 'tempo':
      return {
        key: 'installing_opentelemetry',
        values: {
          resources: k8sResources,
          tempo: { resources: k8sResources },
          tempoQuery: { resources: k8sResources },
        },
      }
    case 'jaeger':
      return {
        key: 'installing_opentelemetry',
        values: {
          resources: k8sResources,
          allInOne: { resources: k8sResources },
          agent: { resources: k8sResources },
          collector: { resources: k8sResources },
          query: { resources: k8sResources },
        },
      }
    case 'opentelemetry':
      return { key: 'installing_opentelemetry', values: { resources: k8sResources } }
    // 수집기는 추적 저장소와 릴리스가 달라 설치 단계도 다르다. 여기서 같은
    // 키를 쓰면 자원 설정이 엉뚱한 릴리스에 실린다.
    case 'opentelemetry-collector':
      return { key: 'installing_otel_collector', values: { resources: k8sResources } }
    default:
      return null
  }
}

// GATEWAY_ROUTE_TIMEOUT_SECONDS 는 도구 라우트 하나가 붙잡을 수 있는 최대 시간이다.
//
// 앞단 ingress 의 proxy 타임아웃과 같은 값이다(gateway-bridge.go). 두 관문의
// 값이 갈라지면 짧은 쪽에서 끊기는데, 어느 쪽이 끊었는지는 로그에 드러나지 않는다.
const GATEWAY_ROUTE_TIMEOUT_SECONDS = 600

export function buildGatewayManifest(draft: StackConfigDraft, manifestTools: ManifestToolEntry[]): string {
  const namespace = draft.namespace.trim() || 'nullus'
  const stackName = draft.stackName || 'nullus-stack'
  const accessDomain = normalizeAccessDomain(draft.accessDomain || `${stackName}.internal`)
  const gatewayName = `${stackName}-gateway`
  const tlsEnabled = draft.accessDomainTls.enabled
  const tlsSecretName = draft.accessDomainTls.secretName.trim()
  const tlsSecretNamespace = draft.accessDomainTls.secretNamespace.trim()
  const tlsIssuerName = draft.accessDomainTls.issuerName.trim() || 'nullus-ca-issuer'
  const requiresReferenceGrant = tlsEnabled && tlsSecretName.length > 0 && tlsSecretNamespace.length > 0 && tlsSecretNamespace !== namespace

  const rules = manifestTools
    .filter((tool) => tool.toolId !== GATEWAY_MANIFEST_ID)
    .map((tool) => {
      const backend = gatewayBackendForTool(tool.toolId)
      return {
        host: `${tool.toolId}.${accessDomain}`,
        http: {
          paths: [
            {
              path: '/',
              pathType: 'Prefix',
              backend: {
                service: {
                  name: backend.serviceName,
                  port: { number: backend.port },
                },
              },
            },
          ],
        },
      }
    })

  const includesOpenSearch = manifestTools.some((tool) => tool.toolId === 'opensearch')

  const gateway = {
    apiVersion: 'gateway.networking.k8s.io/v1',
    kind: 'Gateway',
    metadata: {
      name: gatewayName,
      namespace,
      labels: {
        'nullus.io/stack-name': stackName,
        'nullus.io/type': 'gateway',
      },
    },
    spec: {
      gatewayClassName: 'envoy',
      listeners: [
        {
          name: 'http',
          protocol: 'HTTP',
          port: 80,
          hostname: `*.${accessDomain}`,
          allowedRoutes: {
            namespaces: {
              // 배포된 앱의 HTTPRoute 는 앱이 서는 네임스페이스에 생긴다. 라우트는
              // 같은 네임스페이스의 Service 를 가리켜야 하므로 그 자리를 옮길 수 없다.
              //
              // Same 으로 두면 그 라우트가 게이트웨이에 붙지 못한다. 스택 도구들은
              // 같은 네임스페이스라 잘 열리고 배포된 앱만 안 열리는데, 어느 쪽도
              // 오류를 내지 않아 원인이 드러나지 않는다 — Argo CD 는 Synced/Healthy
              // 인데 주소만 404 로 끝난다.
              from: 'All',
            },
          },
        },
        ...(tlsEnabled && tlsSecretName
          ? [
              {
                name: 'https',
                protocol: 'HTTPS',
                port: 443,
                hostname: `*.${accessDomain}`,
                tls: {
                  mode: 'Terminate',
                  certificateRefs: [
                    {
                      kind: 'Secret',
                      name: tlsSecretName,
                      ...(tlsSecretNamespace ? { namespace: tlsSecretNamespace } : {}),
                    },
                  ],
                },
                allowedRoutes: {
                  namespaces: {
                    // http 리스너와 같은 이유다. 한쪽만 열어 두면 앱이 http 로만
                    // 열리고 그 사실이 어디에도 드러나지 않는다.
                    from: 'All',
                  },
                },
              },
            ]
          : []),
      ],
    },
  }

  const httpRoutes = rules.map((rule) => {
    const host = rule.host
    const backendService = rule.http.paths[0]?.backend?.service
    const backendServiceName = backendService?.name
    const backendServicePort = backendService?.port?.number ?? 80

    return {
      apiVersion: 'gateway.networking.k8s.io/v1',
      kind: 'HTTPRoute',
      metadata: {
        name: `${String(backendServiceName).replace(/-svc$/, '')}-route`,
        namespace,
        labels: {
          'nullus.io/stack-name': stackName,
          'nullus.io/type': 'gateway-route',
        },
      },
      spec: {
        parentRefs: [
          {
            name: gatewayName,
          },
        ],
        hostnames: [host],
        rules: [
          {
            matches: [
              {
                path: {
                  type: 'PathPrefix',
                  value: '/',
                },
              },
            ],
            // Envoy 의 기본 라우트 타임아웃은 15초다. 이미지 layer push 와
            // git push 는 그보다 오래 걸리는 **단일 요청**이라, 정해 두지 않으면
            // 게이트웨이가 중간에 끊는다 — docker 는 그것을 재시도로 받아 모든
            // layer 가 끝없이 "Retrying in N seconds" 를 반복한다.
            //
            // backendRequest 는 request 보다 클 수 없다. 크면 Gateway API 가
            // 라우트를 거부해 도구가 통째로 열리지 않는다.
            timeouts: {
              request: `${GATEWAY_ROUTE_TIMEOUT_SECONDS}s`,
              backendRequest: `${GATEWAY_ROUTE_TIMEOUT_SECONDS}s`,
            },
            backendRefs: [
              {
                name: backendServiceName,
                port: backendServicePort,
              },
            ],
          },
        ],
      },
    }
  })

  const referenceGrant = requiresReferenceGrant
    ? {
        apiVersion: 'gateway.networking.k8s.io/v1beta1',
        kind: 'ReferenceGrant',
        metadata: {
          name: `${stackName}-tls-secret-grant`,
          namespace: tlsSecretNamespace,
          labels: {
            'nullus.io/stack-name': stackName,
            'nullus.io/type': 'gateway-reference-grant',
          },
        },
        spec: {
          from: [
            {
              group: 'gateway.networking.k8s.io',
              kind: 'Gateway',
              namespace,
            },
          ],
          to: [
            {
              group: '',
              kind: 'Secret',
              name: tlsSecretName,
            },
          ],
        },
      }
    : null

  const certificate = tlsEnabled && tlsSecretName
    ? {
        apiVersion: 'cert-manager.io/v1',
        kind: 'Certificate',
        metadata: {
          name: `${stackName}-wildcard-cert`,
          namespace: tlsSecretNamespace || namespace,
          labels: {
            'nullus.io/stack-name': stackName,
            'nullus.io/type': 'gateway-certificate',
          },
        },
        spec: {
          secretName: tlsSecretName,
          commonName: accessDomain,
          dnsNames: [accessDomain, `*.${accessDomain}`],
          issuerRef: {
            name: tlsIssuerName,
            kind: 'ClusterIssuer',
          },
        },
      }
    : null

  const gatewayDocuments = [
    YAML.stringify(gateway, { indent: 2, lineWidth: 0 }),
    ...httpRoutes.map((route) => YAML.stringify(route, { indent: 2, lineWidth: 0 })),
    ...(includesOpenSearch
      ? [
          YAML.stringify(buildOpenSearchBackendTLSPolicy(namespace, stackName, accessDomain), { indent: 2, lineWidth: 0 }),
        ]
      : []),
    ...(certificate ? [YAML.stringify(certificate, { indent: 2, lineWidth: 0 })] : []),
    ...(referenceGrant ? [YAML.stringify(referenceGrant, { indent: 2, lineWidth: 0 })] : []),
  ]

  return gatewayDocuments.join('\n---\n')
}

export function createDeployScript(
  draft: StackConfigDraft,
  manifestTools: ManifestToolEntry[],
  manifestByTool: Record<string, string>,
  noneLabel: string
): string {
  const stackName = draft.stackName || 'nullus-stack'
  const namespace = draft.namespace.trim() || 'nullus'
  const accessDomain = normalizeAccessDomain(draft.accessDomain || `${stackName}.internal`)
  const clusterContext = draft.clusterId ?? ''
  const tlsEnabled = draft.accessDomainTls.enabled
  const tlsSecretName = draft.accessDomainTls.secretName.trim() || `${stackName}-wildcard-tls`
  const tlsSecretNamespace = draft.accessDomainTls.secretNamespace.trim() || namespace
  const tlsIssuerName = draft.accessDomainTls.issuerName.trim() || 'nullus-ca-issuer'

  const deployBlocks = manifestTools.flatMap((tool, index) => {
    const blockHeader = [`# ${index + 1}. ${toolLabel(tool.toolId, noneLabel)} (${tool.installType.toUpperCase()})`, `# roles: ${tool.roles.join(', ')}`]
    const manifestText = (manifestByTool[tool.toolId] ?? '').trimEnd()

    if (tool.installType === 'helm') {
      const meta = getHelmMeta(tool.toolId)
      const chartVersion = tool.chartVersion || getToolChartVersion(tool.toolId) || tool.toolVersion
      const valuesPath = `.nullus/generated-values/${tool.toolId}.values.yaml`
      const delimiter = `NULLUS_VALUES_EOF_${index + 1}`
      return [
        ...blockHeader,
        `cat <<'${delimiter}' > "${valuesPath}"`,
        ...manifestText.split('\n'),
        delimiter,
        `helm repo add ${tool.toolId} ${meta.repoUrl}`,
        `helm upgrade --install ${tool.toolId} ${meta.chartName} --namespace ${namespace} --create-namespace -f "${valuesPath}" --version ${chartVersion}`,
        '',
      ]
    }

    const manifestPath = `.nullus/generated-manifests/${tool.toolId}.yaml`
    const delimiter = `NULLUS_MANIFEST_EOF_${index + 1}`

    return [
      ...blockHeader,
      `cat <<'${delimiter}' > "${manifestPath}"`,
      ...manifestText.split('\n'),
      delimiter,
      `kubectl apply -n ${namespace} -f "${manifestPath}"`,
      '',
    ]
  })

  return [
    '#!/usr/bin/env bash',
    '# Nullus Stack Deploy Script (generated from current Stack Install options)',
    `# stack: ${stackName}`,
    `# namespace: ${namespace}`,
    `# access-domain: ${accessDomain}`,
    `# access-domain-tls: ${tlsEnabled ? `enabled (${tlsSecretNamespace}/${tlsSecretName})` : 'disabled'}`,
    `# cluster-context: ${clusterContext || '(current context)'}`,
    '',
    'set -euo pipefail',
    '',
    clusterContext ? `kubectl config use-context "${clusterContext}"` : '# using current kubectl context',
    `kubectl create namespace ${namespace} --dry-run=client -o yaml | kubectl apply -f -`,
    'mkdir -p .nullus/generated-values .nullus/generated-manifests',
    '',
    '# storage plan',
    `# plan_mode=${draft.storage.planMode}`,
    `# database=${draft.storage.database.mode}:${draft.storage.database.providerOrEngine}:${draft.storage.database.version}`,
    `# object_storage=${draft.storage.objectStorage.mode}:${draft.storage.objectStorage.providerOrEngine}:${draft.storage.objectStorage.version}`,
    '',
    '# resource planning inputs',
    `# developers=${draft.resources.developerCount}, runners=${draft.resources.concurrentRunners}, commitsPerDay=${draft.resources.commitsPerDay}`,
    `# buildFrequency=${draft.resources.buildFrequency}, currency=${draft.resources.currency}`,
    '',
    ...(tlsEnabled
      ? [
          '# TLS via cert-manager (manual openssl/secret creation is not required)',
          `cat <<'NULLUS_TLS_CERT_EOF' > ".nullus/generated-manifests/${stackName}-tls-certificate.yaml"`,
          'apiVersion: cert-manager.io/v1',
          'kind: Certificate',
          'metadata:',
          `  name: ${stackName}-wildcard-cert`,
          `  namespace: ${tlsSecretNamespace}`,
          'spec:',
          `  secretName: ${tlsSecretName}`,
          `  commonName: ${accessDomain}`,
          '  dnsNames:',
          `    - ${accessDomain}`,
          `    - *.${accessDomain}`,
          '  issuerRef:',
          `    name: ${tlsIssuerName}`,
          '    kind: ClusterIssuer',
          'NULLUS_TLS_CERT_EOF',
          `kubectl apply -f ".nullus/generated-manifests/${stackName}-tls-certificate.yaml"`,
          ...(tlsSecretNamespace !== namespace
            ? [
              '# If TLS Secret namespace differs from Gateway namespace, apply ReferenceGrant too.',
                `cat <<'NULLUS_TLS_REFGRANT_EOF' > ".nullus/generated-manifests/${stackName}-tls-reference-grant.yaml"`,
                'apiVersion: gateway.networking.k8s.io/v1beta1',
                'kind: ReferenceGrant',
                'metadata:',
                `  name: ${stackName}-tls-secret-grant`,
                `  namespace: ${tlsSecretNamespace}`,
                'spec:',
                '  from:',
                '    - group: gateway.networking.k8s.io',
                '      kind: Gateway',
                `      namespace: ${namespace}`,
                '  to:',
                '    - group: ""',
                '      kind: Secret',
                `      name: ${tlsSecretName}`,
                'NULLUS_TLS_REFGRANT_EOF',
                `kubectl apply -f ".nullus/generated-manifests/${stackName}-tls-reference-grant.yaml"`,
              ]
            : []),
          '',
        ]
      : []),
    ...deployBlocks,
    'echo "✅ Nullus stack deploy script completed"',
  ].join('\n')
}
