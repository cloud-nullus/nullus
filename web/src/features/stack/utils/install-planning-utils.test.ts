import { describe, expect, it } from 'vitest'
import { calculateMultipliers } from './install-planning-utils'

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
