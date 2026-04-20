import { describe, it, expect } from 'vitest'
import { extractDeployCompatError } from './deploy-error'

describe('extractDeployCompatError', () => {
  it('parses DEPLOY_COMPAT_FAIL with issue list', () => {
    const err = {
      status: 400,
      details: {
        error: {
          code: 'DEPLOY_COMPAT_FAIL',
          verdict: {
            issues: [
              { code: 'K8S_RANGE_MISMATCH', message: 'cluster 1.32 out of [1.26,1.31]' },
              { code: 'TOOL_ARCH_UNSUPPORTED', message: 'argo 2.12 lacks arm64' },
            ],
          },
        },
      },
    }
    const gate = extractDeployCompatError(err)
    expect(gate).not.toBeNull()
    expect(gate?.code).toBe('DEPLOY_COMPAT_FAIL')
    expect(gate?.issueLines).toEqual([
      '[K8S_RANGE_MISMATCH] cluster 1.32 out of [1.26,1.31]',
      '[TOOL_ARCH_UNSUPPORTED] argo 2.12 lacks arm64',
    ])
  })

  it('parses DEPLOY_COMPAT_WARN_UNACK even when issues omit code', () => {
    const err = {
      details: {
        error: {
          code: 'DEPLOY_COMPAT_WARN_UNACK',
          verdict: {
            issues: [{ message: 'untested combination' }],
          },
        },
      },
    }
    const gate = extractDeployCompatError(err)
    expect(gate?.code).toBe('DEPLOY_COMPAT_WARN_UNACK')
    expect(gate?.issueLines).toEqual(['untested combination'])
  })

  it('returns null for non-compat errors', () => {
    const err = {
      details: { error: { code: 'CLUSTER_UNREACHABLE', message: 'timeout' } },
    }
    expect(extractDeployCompatError(err)).toBeNull()
  })

  it('returns null for malformed body', () => {
    expect(extractDeployCompatError(null)).toBeNull()
    expect(extractDeployCompatError(undefined)).toBeNull()
    expect(extractDeployCompatError({})).toBeNull()
    expect(extractDeployCompatError({ details: 'oops' })).toBeNull()
    expect(extractDeployCompatError({ details: { error: null } })).toBeNull()
  })
})
