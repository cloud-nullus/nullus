import { describe, expect, it } from 'vitest'
import { IN_CLUSTER_OBJECT_STORAGE_IDS, STORAGE_PROVIDER_OPTIONS, isInClusterObjectStorage } from './install-constants'
import { buildGatewayManifest, buildHelmStepResourceOverride } from './install-manifest-builders'
import { compatibilityRequestedTools } from '../pages/stack-install-page'
import { buildInstallOverridesFromTemplate } from './template-overrides'
import { useStackConfigStore } from '../stores/stack-config-store'
import type { StackTemplate } from '../../../types'

describe('stack object storage backend', () => {
  it('should offer every in-cluster S3 implementation on the Storage tab', () => {
    const ids = STORAGE_PROVIDER_OPTIONS.objectStorage.map((option) => option.id)
    for (const id of IN_CLUSTER_OBJECT_STORAGE_IDS) {
      expect(ids).toContain(id)
    }
    expect(isInClusterObjectStorage('SeaweedFS')).toBe(true)
    expect(isInClusterObjectStorage('s3')).toBe(false)
  })

  // 서버(withoutInterchangeableSlots)와 같은 규칙이다. 매트릭스는 MinIO 로 적혀 있으므로
  // 이 슬롯이 대조에 들어가면 SeaweedFS 를 고른 조합이 전부 "일치하는 매트릭스 없음" 이 된다.
  it('should not split matrix matching by in-cluster object storage implementation', () => {
    const tools = compatibilityRequestedTools([
      { slot: 'artifacts.sourceRepository', toolKey: 'gitlab' },
      { slot: 'artifacts.storageBackend', toolKey: 'seaweedfs' },
    ])
    expect(tools).toEqual({ source_repository: 'GitLab CE' })

    const external = compatibilityRequestedTools([{ slot: 'artifacts.storageBackend', toolKey: 's3' }])
    expect(external).toHaveProperty('storage_backend')
  })

  it('should route SeaweedFS resource plans to the all-in-one pod', () => {
    const override = buildHelmStepResourceOverride('seaweedfs', {
      cpuRequest: 0.25,
      cpuLimit: 1,
      memoryRequestGi: 0.5,
      memoryLimitGi: 2,
      storageRequestGi: 50,
      storageLimitGi: 100,
    })
    expect(override?.key).toBe('installing_seaweedfs')
    expect(override?.values).toHaveProperty('allInOne.resources')
  })

  // 요청은 Storage 탭의 값을 storage_backend 로 보낸다. 템플릿 값이 거기 실리지 않으면
  // SeaweedFS 템플릿을 골라도 기본값(minio)이 이겨 MinIO 가 설치된다.
  it('should carry the template object storage choice into the Storage tab', () => {
    const template = {
      id: 'gitlab-seaweedfs',
      name: 'GitLab + SeaweedFS',
      tools: [],
      toolDetails: [
        { category: 'source_repository', name: 'GitLab CE' },
        { category: 'storage_backend', name: 'SeaweedFS', app_version: '4.48' },
      ],
    } as unknown as StackTemplate
    const overrides = buildInstallOverridesFromTemplate(template)
    expect(overrides.artifacts?.storageBackend.tool).toBe('seaweedfs')

    useStackConfigStore.getState().loadFromTemplate(template.id, overrides)
    const { objectStorage } = useStackConfigStore.getState().draft.storage
    expect(objectStorage.providerOrEngine).toBe('seaweedfs')
    expect(objectStorage.version).toBe('4.48')
  })

  // 화면이 만든 게이트웨이가 서버 기본값을 대신한다. SeaweedFS 는 화면(콘솔)이 없으므로 라우트를
  // 만들면 없는 서비스(seaweedfs-svc)를 가리키는 죽은 라우트가 설치된다.
  it('should not route SeaweedFS through the gateway while keeping the MinIO console', () => {
    useStackConfigStore.getState().resetConfig()
    const draft = { ...useStackConfigStore.getState().draft, stackName: 'swfs-stack', accessDomain: 'nullus.local' }
    const entry = (toolId: string) => ({
      toolId,
      toolLabel: toolId,
      installType: 'helm' as const,
      toolVersion: '1',
      hasVersionConflict: false,
      roles: [],
      sourceToolIds: [toolId],
      sourceVersions: ['1'],
    })
    expect(buildGatewayManifest(draft, [entry('gitlab'), entry('seaweedfs')])).not.toContain('seaweedfs.nullus.local')
    expect(buildGatewayManifest(draft, [entry('gitlab'), entry('minio')])).toContain('minio.nullus.local')
  })
})
