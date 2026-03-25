import { describe, it, expect, beforeEach } from 'vitest'
import { useStackConfigStore } from './stack-config-store'

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
    expect(draft.artifacts.packageRegistry.version).toBe('18.5.1')
    expect(draft.pipeline.cdTool.version).toBe('v2.8.3')
    expect(draft.storage.planMode).toBe('integrated-create')
    expect(draft.storage.database.mode).toBe('create')
    expect(draft.storage.objectStorage.mode).toBe('create')
    expect(draft.storage.database.endpoint).toBe('postgres.shared.svc:5432')
    expect(draft.storage.objectStorage.endpoint).toBe('http://minio.shared.svc:9000')
    expect(draft.storage.database.authId).toBe('nullus_app')
    expect(draft.storage.database.authPasswordKey).toBe('password')
    expect(draft.storage.objectStorage.authId).toBe('nullus_access_key')
    expect(draft.storage.objectStorage.authPasswordKey).toBe('secretKey')
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
      expect(useStackConfigStore.getState().draft.pipeline.cicdPlatform.version).toBe('v0.9.0')
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
