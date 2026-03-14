import { describe, it, expect, beforeEach } from 'vitest'
import { useStackConfigStore } from './stack-config-store'

beforeEach(() => {
  useStackConfigStore.getState().resetConfig()
})

describe('stack-config-store', () => {
  it('initial state has default values', () => {
    const { draft, isDirty } = useStackConfigStore.getState()
    expect(draft.stackName).toBe('')
    expect(draft.selectedTemplateId).toBeNull()
    expect(draft.activeTab).toBe('artifacts')
    expect(isDirty).toBe(false)
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
