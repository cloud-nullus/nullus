import { describe, it, expect, beforeEach } from 'vitest'
import { getToolAppVersion, useStackConfigStore } from './stack-config-store'

beforeEach(() => {
  useStackConfigStore.getState().resetConfig()
})

describe('stack-config-store', () => {
  it('initial state has default values', () => {
    const { draft, isDirty } = useStackConfigStore.getState()
    expect(draft.stackName).toBe('')
    expect(draft.accessDomain).toBe('')
    expect(draft.accessDomainTls.enabled).toBe(false)
    expect(draft.accessDomainTls.secretName).toBe('nullus-wildcard-tls')
    expect(draft.accessDomainTls.secretNamespace).toBe('nullus')
    expect(draft.accessDomainTls.issuerName).toBe('nullus-ca-issuer')
    expect(draft.selectedTemplateId).toBeNull()
    expect(draft.activeTab).toBe('artifacts')
    // 버전은 리터럴로 다시 적지 않는다. 적어 두면 표와 이 테스트가 각자
    // 출처가 되어 조용히 갈라진다 — 실제로 표가 백엔드 상수보다 뒤처져 있었고
    // 여기에 박힌 숫자가 그 상태를 "정상"으로 굳히고 있었다.
    expect(draft.artifacts.packageRegistry.version).toBe(getToolAppVersion('gitlab'))
    expect(draft.pipeline.cdTool.version).toBe(getToolAppVersion('argocd'))
    expect(draft.monitoring.visualizations.map((item) => item.tool)).toEqual(['grafana'])
    expect(draft.storage.planMode).toBe('integrated-create')
    expect(draft.storage.database.mode).toBe('create')
    expect(draft.storage.objectStorage.mode).toBe('create')
    expect(draft.storage.database.endpoint).toBe('host.docker.internal:5433')
    expect(draft.storage.objectStorage.endpoint).toBe('http://host.docker.internal:9000')
    expect(draft.storage.database.authId).toBe('nullus')
    expect(draft.storage.database.authPasswordKey).toBe('nullus_dev')
    expect(draft.storage.objectStorage.authId).toBe('nullus')
    expect(draft.storage.objectStorage.authPasswordKey).toBe('nullus_dev')
    expect(isDirty).toBe(false)
  })

  describe('storage actions', () => {
    it('updateStorage changes plan mode and marks dirty', () => {
      useStackConfigStore.getState().updateStorage({ planMode: 'existing-all' })
      const { draft, isDirty } = useStackConfigStore.getState()
      expect(draft.storage.planMode).toBe('existing-all')
      expect(isDirty).toBe(true)
    })

    it('updateStorageTarget updates database fields without mutating object storage', () => {
      const beforeObjectStorage = useStackConfigStore.getState().draft.storage.objectStorage
      useStackConfigStore.getState().updateStorageTarget('database', {
        mode: 'create',
        providerOrEngine: 'postgres',
        version: '17',
        size: 'large',
        endpoint: 'db.prod.svc:5432',
        resourceName: 'prod',
        accessSecretRef: 'prod-db-secret',
        authId: 'prod_app',
        authPasswordKey: 'password',
      })

      const { draft } = useStackConfigStore.getState()
      expect(draft.storage.database.mode).toBe('create')
      expect(draft.storage.database.providerOrEngine).toBe('postgres')
      expect(draft.storage.database.version).toBe('17')
      expect(draft.storage.database.size).toBe('large')
      expect(draft.storage.database.endpoint).toBe('db.prod.svc:5432')
      expect(draft.storage.database.resourceName).toBe('prod')
      expect(draft.storage.database.accessSecretRef).toBe('prod-db-secret')
      expect(draft.storage.database.authId).toBe('prod_app')
      expect(draft.storage.database.authPasswordKey).toBe('password')
      expect(draft.storage.objectStorage).toEqual(beforeObjectStorage)
    })

    it('updateStorageTarget updates objectStorage existing reference', () => {
      useStackConfigStore.getState().updateStorageTarget('objectStorage', {
        existingRef: 'team-a-shared-minio',
      })

      expect(useStackConfigStore.getState().draft.storage.objectStorage.existingRef).toBe('team-a-shared-minio')
    })
  })

  describe('setTool', () => {
    it('updates artifacts section tool and marks dirty', () => {
      useStackConfigStore.getState().setTool('artifacts', 'packageRegistry', { tool: 'nexus', version: '3.x' })
      const { draft, isDirty } = useStackConfigStore.getState()
      expect(draft.artifacts.packageRegistry.tool).toBe('nexus')
      expect(draft.artifacts.packageRegistry.version).toBe('3.x')
      expect(isDirty).toBe(true)
    })

    it('updates pipeline section tool', () => {
      useStackConfigStore.getState().setTool('pipeline', 'cicdPlatform', { tool: 'github-actions', version: 'latest' })
      expect(useStackConfigStore.getState().draft.pipeline.cicdPlatform.tool).toBe('github-actions')
      // GitHub Actions 는 외부 SaaS 라 설치할 차트가 없다. 예전에는 이 자리에
      // actions-runner-controller 버전이 들어갔지만, 백엔드가 external 로 표시해
      // 설치를 건너뛰므로 러너 차트 버전을 노출하면 설치 계획이 실제와 어긋난다.
      expect(useStackConfigStore.getState().draft.pipeline.cicdPlatform.version).toBe('external')
    })

    it('updates monitoring section tool', () => {
      useStackConfigStore.getState().setTool('monitoring', 'collection', { tool: 'datadog', version: 'latest' })
      expect(useStackConfigStore.getState().draft.monitoring.collection.tool).toBe('datadog')
    })

    it('updates logging section tool', () => {
      useStackConfigStore.getState().setTool('logging', 'search', { tool: 'elasticsearch', version: '8.x' })
      expect(useStackConfigStore.getState().draft.logging.search.tool).toBe('elasticsearch')
    })

    it('does not mutate other fields in the section', () => {
      const before = useStackConfigStore.getState().draft.artifacts.sourceRepository.tool
      useStackConfigStore.getState().setTool('artifacts', 'packageRegistry', { tool: 'jfrog', version: 'latest' })
      expect(useStackConfigStore.getState().draft.artifacts.sourceRepository.tool).toBe(before)
    })

    it('setMonitoringVisualizations keeps primary visualization in sync', () => {
      useStackConfigStore.getState().setMonitoringVisualizations([
        { tool: 'grafana', version: 'latest' },
        { tool: 'kibana', version: 'latest' },
      ])
      const { monitoring } = useStackConfigStore.getState().draft
      expect(monitoring.visualization.tool).toBe('grafana')
      expect(monitoring.visualizations.map((item) => item.tool)).toEqual(['grafana', 'kibana'])
    })
  })

  describe('stack name and access domain', () => {
    it('setStackName updates default access domain automatically', () => {
      useStackConfigStore.getState().setStackName('team-stack')
      const { draft } = useStackConfigStore.getState()
      expect(draft.accessDomain).toBe('team-stack.internal')
    })

    it('manual accessDomain is preserved on later stackName changes', () => {
      useStackConfigStore.getState().setStackName('team-stack')
      useStackConfigStore.getState().setAccessDomain('custom.company.internal')
      useStackConfigStore.getState().setStackName('next-stack')
      const { draft } = useStackConfigStore.getState()
      expect(draft.accessDomain).toBe('custom.company.internal')
    })

    it('normalizes common typo intenral to internal', () => {
      useStackConfigStore.getState().setAccessDomain('nullus-devsecops-stack.intenral')
      const { draft } = useStackConfigStore.getState()
      expect(draft.accessDomain).toBe('nullus-devsecops-stack.internal')
    })

    it('setStackName updates default TLS secret name automatically', () => {
      useStackConfigStore.getState().setStackName('team-stack')
      expect(useStackConfigStore.getState().draft.accessDomainTls.secretName).toBe('team-stack-wildcard-tls')
    })

    it('updateAccessDomainTls updates tls options and marks dirty', () => {
      useStackConfigStore.getState().updateAccessDomainTls({
        enabled: true,
        secretName: 'corp-wildcard',
        secretNamespace: 'kube-system',
        issuerName: 'corp-cluster-issuer',
      })
      const { draft, isDirty } = useStackConfigStore.getState()
      expect(draft.accessDomainTls.enabled).toBe(true)
      expect(draft.accessDomainTls.secretName).toBe('corp-wildcard')
      expect(draft.accessDomainTls.secretNamespace).toBe('kube-system')
      expect(draft.accessDomainTls.issuerName).toBe('corp-cluster-issuer')
      expect(isDirty).toBe(true)
    })
  })

  describe('loadFromTemplate', () => {
    it('sets selectedTemplateId and resets dirty flag', () => {
      useStackConfigStore.getState().setStackName('dirty')
      useStackConfigStore.getState().loadFromTemplate('tpl-001')
      const { draft, isDirty } = useStackConfigStore.getState()
      expect(draft.selectedTemplateId).toBe('tpl-001')
      expect(isDirty).toBe(false)
    })

    it('applies overrides on top of defaults', () => {
      useStackConfigStore.getState().loadFromTemplate('tpl-002', { stackName: 'my-stack' })
      expect(useStackConfigStore.getState().draft.stackName).toBe('my-stack')
      // defaults preserved
      expect(useStackConfigStore.getState().draft.activeTab).toBe('artifacts')
    })

    it('resets all fields to defaults when no overrides given', () => {
      useStackConfigStore.getState().setTool('pipeline', 'cdTool', { tool: 'flux', version: 'latest' })
      useStackConfigStore.getState().loadFromTemplate('tpl-003')
      expect(useStackConfigStore.getState().draft.pipeline.cdTool.tool).toBe('argocd')
    })

    it('loads empty template with all tool selections cleared', () => {
      useStackConfigStore.getState().loadFromTemplate('empty-template-v1')
      const { draft } = useStackConfigStore.getState()

      expect(draft.selectedTemplateId).toBe('empty-template-v1')
      expect(draft.artifacts.packageRegistry.tool).toBe('')
      expect(draft.artifacts.sourceRepository.tool).toBe('')
      expect(draft.artifacts.containerRegistry.tool).toBe('')
      expect(draft.artifacts.storageBackend.tool).toBe('')
      expect(draft.pipeline.cicdPlatform.tool).toBe('')
      expect(draft.pipeline.cdTool.tool).toBe('')
      expect(draft.monitoring.collection.tool).toBe('')
      expect(draft.monitoring.visualization.tool).toBe('')
      expect(draft.monitoring.visualizations).toEqual([])
      expect(draft.logging.search.tool).toBe('')
      expect(draft.logging.traceLayer.tool).toBe('')
      expect(draft.logging.traceExporter.tool).toBe('')
      expect(draft.storage.planMode).toBe('none')
    })
  })

  describe('resetConfig', () => {
    it('resets state to initial defaults', () => {
      useStackConfigStore.getState().setStackName('test')
      useStackConfigStore.getState().setTool('artifacts', 'packageRegistry', { tool: 'nexus', version: 'latest' })
      useStackConfigStore.getState().resetConfig()
      const { draft, isDirty } = useStackConfigStore.getState()
      expect(draft.stackName).toBe('')
      expect(draft.artifacts.packageRegistry.tool).toBe('gitlab')
      expect(isDirty).toBe(false)
    })

    it('resets selectedTemplateId to null', () => {
      useStackConfigStore.getState().loadFromTemplate('tpl-xyz')
      useStackConfigStore.getState().resetConfig()
      expect(useStackConfigStore.getState().draft.selectedTemplateId).toBeNull()
    })
  })
})
