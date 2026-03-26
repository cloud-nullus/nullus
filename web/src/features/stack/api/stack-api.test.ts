import { describe, expect, it } from 'vitest'
import type { CreateStackRequest } from '../../../types'
import { toCreateStackBody } from './stack-api'

const baseRequest: CreateStackRequest = {
  templateId: 'gitlab-argocd-v1',
  clusterId: 'cluster-1',
  namespace: 'nullus',
  stackName: 'size-mapping-test',
  accessDomain: 'size-mapping-test.internal',
  artifacts: {
    packageRegistry: { tool: 'gitlab', version: '1.0.0' },
    sourceRepository: { tool: 'gitlab', version: '1.0.0' },
    containerRegistry: { tool: 'harbor', version: '1.0.0' },
    storageBackend: { tool: 'minio', version: '1.0.0' },
  },
  pipeline: {
    cicdPlatform: { tool: 'gitlab-ci', version: '1.0.0' },
    cdTool: { tool: 'argocd', version: '1.0.0' },
  },
  monitoring: {
    collection: { tool: 'prometheus', version: '1.0.0' },
    visualization: { tool: 'grafana', version: '1.0.0' },
  },
  logging: {
    collection: { tool: 'opensearch', version: '1.0.0' },
    search: { tool: 'opensearch', version: '1.0.0' },
  },
  resources: {
    developerCount: 10,
    concurrentRunners: 5,
    commitsPerDay: 50,
    buildFrequency: 'medium',
    currency: 'KRW',
  },
  storage: {
    planMode: 'integrated-create',
    database: {
      mode: 'create',
      existingRef: '',
      endpoint: '',
      resourceName: 'app-db',
      accessSecretRef: 'db-secret',
      authId: 'app',
      authPasswordKey: 'password',
      providerOrEngine: 'postgres',
      version: '16',
      size: 'medium',
    },
    objectStorage: {
      mode: 'create',
      existingRef: '',
      endpoint: '',
      resourceName: 'app-storage',
      accessSecretRef: 'storage-secret',
      authId: 'access',
      authPasswordKey: 'secret',
      providerOrEngine: 'minio',
      version: '2024.1',
      size: 'large',
    },
  },
}

describe('toCreateStackBody storage size mapping', () => {
  it('maps create-mode size enums to numeric Gi values', () => {
    const body = toCreateStackBody(baseRequest)
    const storage = body.config.storage

    expect(storage?.database.size).toBe(50)
    expect(storage?.object_storage.size).toBe(300)
  })

  it('omits size when storage target mode is existing', () => {
    const request: CreateStackRequest = {
      ...baseRequest,
      storage: {
        ...baseRequest.storage!,
        database: { ...baseRequest.storage!.database, mode: 'existing' },
        objectStorage: { ...baseRequest.storage!.objectStorage, mode: 'existing' },
      },
    }

    const body = toCreateStackBody(request)
    const storage = body.config.storage

    expect(storage?.database.size).toBeUndefined()
    expect(storage?.object_storage.size).toBeUndefined()
  })
})
