import { describe, expect, it } from 'vitest'
import { applyMultipliers, calculateMultipliers, roundRecommendedHalf } from './install-planning-utils'

describe('calculateMultipliers', () => {
  it('allows a compact local floor for kind-sized workloads', () => {
    const values = calculateMultipliers('local', 'monitoring.collection', {
      metricsTargets: 20,
      scrapeIntervalSec: 75,
      retentionDays: 3,
    })

    expect(values.cpu).toBeGreaterThanOrEqual(0.15)
    expect(values.cpu).toBeLessThan(0.5)
    expect(values.memory).toBeGreaterThanOrEqual(0.15)
    expect(values.memory).toBeLessThan(0.5)
  })

  it('retains the hosted profile minimum floor', () => {
    const values = calculateMultipliers('startup', 'monitoring.collection', {
      metricsTargets: 20,
      scrapeIntervalSec: 75,
      retentionDays: 3,
    })

    expect(values.cpu).toBeGreaterThanOrEqual(0.5)
    expect(values.memory).toBeGreaterThanOrEqual(0.5)
  })
})

describe('recommended resource rounding', () => {
  it('rounds positive recommendations to 0.5 increments with a positive floor', () => {
    expect(roundRecommendedHalf(0.24)).toBe(0.5)
    expect(roundRecommendedHalf(0.74)).toBe(0.5)
    expect(roundRecommendedHalf(0.75)).toBe(1)
    expect(roundRecommendedHalf(2.26)).toBe(2.5)
  })

  it('quantizes multiplier recommendations while preserving zero-valued resources', () => {
    expect(applyMultipliers({
      cpu_request: 1,
      cpu_limit: 2,
      memory_request_gi: 1,
      memory_limit_gi: 2,
      storage_request_gi: 0,
      storage_limit_gi: 0,
    }, {
      cpu: 0.74,
      memory: 1.26,
      storage: 1.4,
    })).toEqual({
      cpuRequest: 0.5,
      cpuLimit: 1.5,
      memoryRequestGi: 1.5,
      memoryLimitGi: 2.5,
      storageRequestGi: 0,
      storageLimitGi: 0,
    })
  })
})
