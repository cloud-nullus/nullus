export type ManifestInstallType = 'helm' | 'yaml'

export type PlanningSlot =
  | 'artifacts.packageRegistry'
  | 'artifacts.sourceRepository'
  | 'artifacts.containerRegistry'
  | 'artifacts.storageBackend'
  | 'pipeline.cicdPlatform'
  | 'pipeline.cdTool'
  | 'monitoring.collection'
  | 'monitoring.visualization'
  | 'logging.search'
  | 'logging.traceLayer'
  | 'logging.traceExporter'

export type PlanningProfile = 'local' | 'startup' | 'standard' | 'enterprise'

export type ResourceVector = {
  cpuRequest: number
  cpuLimit: number
  memoryRequestGi: number
  memoryLimitGi: number
  storageRequestGi: number
  storageLimitGi: number
}

export type ResourceMultipliers = {
  cpu: number
  memory: number
  storage: number
  raw: {
    cpu: number
    memory: number
    storage: number
  }
  clamped: {
    cpu: boolean
    memory: boolean
    storage: boolean
  }
}

export type ResourceUnit = 'Gi' | 'Mi'

export type PlanningRowUnit = {
  memory: ResourceUnit
  storage: ResourceUnit
}

export type PlanningOptionDefinition = {
  key: string
  label: string
  baseline: number
  min: number
  max: number
  step: number
  weight: number
  impact: {
    cpu: number
    memory: number
    storage: number
  }
}

export const PLANNING_PROFILE_LABEL: Record<PlanningProfile, string> = {
  local: 'Local',
  startup: 'Startup',
  standard: 'Standard',
  enterprise: 'Enterprise',
}

export const PLANNING_PROFILES: PlanningProfile[] = ['local', 'startup', 'standard', 'enterprise']

export const PLANNING_OPTION_DEFS: Record<PlanningSlot, PlanningOptionDefinition[]> = {
  'artifacts.packageRegistry': [
    { key: 'registryCallsPerDay', label: 'Registry 호출 수/일', baseline: 3000, min: 500, max: 50000, step: 500, weight: 0.45, impact: { cpu: 1, memory: 0.8, storage: 0.3 } },
    { key: 'avgArtifactSizeMb', label: '평균 패키지 크기(MB)', baseline: 120, min: 10, max: 2000, step: 10, weight: 0.25, impact: { cpu: 0.1, memory: 0.2, storage: 1 } },
    { key: 'retentionDays', label: '보관 기간(일)', baseline: 30, min: 1, max: 365, step: 1, weight: 0.30, impact: { cpu: 0, memory: 0.1, storage: 1 } },
  ],
  'artifacts.sourceRepository': [
    { key: 'activeRepoUsers', label: '활성 Repo 사용자 수', baseline: 20, min: 5, max: 500, step: 1, weight: 0.4, impact: { cpu: 0.8, memory: 0.6, storage: 0.3 } },
    { key: 'repoCount', label: '관리 저장소 수', baseline: 60, min: 5, max: 3000, step: 5, weight: 0.35, impact: { cpu: 0.2, memory: 0.4, storage: 1 } },
    { key: 'dailyPushEvents', label: '일일 Push 이벤트 수', baseline: 250, min: 20, max: 20000, step: 10, weight: 0.25, impact: { cpu: 1, memory: 0.5, storage: 0.4 } },
  ],
  'artifacts.containerRegistry': [
    { key: 'imagePullsPerDay', label: '이미지 Pull 수/일', baseline: 2000, min: 200, max: 100000, step: 100, weight: 0.4, impact: { cpu: 0.8, memory: 0.7, storage: 0.4 } },
    { key: 'newImagePushesPerDay', label: '신규 이미지 Push 수/일', baseline: 180, min: 10, max: 8000, step: 10, weight: 0.35, impact: { cpu: 0.9, memory: 0.6, storage: 0.8 } },
    { key: 'avgImageSizeGb', label: '평균 이미지 크기(GB)', baseline: 1.2, min: 0.1, max: 20, step: 0.1, weight: 0.25, impact: { cpu: 0.1, memory: 0.2, storage: 1 } },
  ],
  'artifacts.storageBackend': [
    { key: 'objectOpsPerDay', label: 'Object 요청 수/일', baseline: 10000, min: 1000, max: 200000, step: 500, weight: 0.45, impact: { cpu: 0.9, memory: 0.8, storage: 0.4 } },
    { key: 'storedDataTb', label: '저장 데이터(TB)', baseline: 1.5, min: 0.1, max: 100, step: 0.1, weight: 0.35, impact: { cpu: 0.1, memory: 0.2, storage: 1 } },
    { key: 'backupFrequencyPerWeek', label: '주간 백업 횟수', baseline: 7, min: 1, max: 30, step: 1, weight: 0.20, impact: { cpu: 0.4, memory: 0.3, storage: 0.7 } },
  ],
  'pipeline.cicdPlatform': [
    { key: 'developers', label: '개발자 수', baseline: 20, min: 1, max: 1000, step: 1, weight: 0.2, impact: { cpu: 0.4, memory: 0.4, storage: 0.2 } },
    { key: 'concurrentRunners', label: '동시 러너 수', baseline: 4, min: 1, max: 400, step: 1, weight: 0.55, impact: { cpu: 1.8, memory: 1.6, storage: 0.7 } },
    { key: 'dailyCommits', label: '일일 커밋 수', baseline: 120, min: 10, max: 10000, step: 10, weight: 0.25, impact: { cpu: 0.8, memory: 0.6, storage: 0.3 } },
  ],
  'pipeline.cdTool': [
    { key: 'deploymentsPerDay', label: '배포 횟수/일', baseline: 40, min: 1, max: 2000, step: 1, weight: 0.5, impact: { cpu: 0.8, memory: 0.6, storage: 0.2 } },
    { key: 'environmentsCount', label: '운영 환경 수', baseline: 4, min: 1, max: 30, step: 1, weight: 0.25, impact: { cpu: 0.4, memory: 0.5, storage: 0.3 } },
    { key: 'rollbackRatePercent', label: '롤백 비율(%)', baseline: 8, min: 0, max: 80, step: 1, weight: 0.25, impact: { cpu: 0.5, memory: 0.6, storage: 0.2 } },
  ],
  'monitoring.collection': [
    { key: 'metricsTargets', label: '모니터링 타겟 수', baseline: 150, min: 20, max: 5000, step: 5, weight: 0.45, impact: { cpu: 0.7, memory: 0.9, storage: 0.4 } },
    { key: 'scrapeIntervalSec', label: '스크랩 주기(초)', baseline: 30, min: 5, max: 120, step: 1, weight: 0.30, impact: { cpu: -0.6, memory: -0.7, storage: -0.2 } },
    { key: 'retentionDays', label: '메트릭 보관 기간(일)', baseline: 15, min: 1, max: 365, step: 1, weight: 0.25, impact: { cpu: 0, memory: 0.2, storage: 1 } },
  ],
  'monitoring.visualization': [
    { key: 'dashboardUsers', label: '대시보드 사용자 수', baseline: 30, min: 5, max: 2000, step: 1, weight: 0.45, impact: { cpu: 0.5, memory: 0.5, storage: 0.1 } },
    { key: 'dashboardCount', label: '대시보드 수', baseline: 40, min: 5, max: 1500, step: 5, weight: 0.30, impact: { cpu: 0.4, memory: 0.6, storage: 0.2 } },
    { key: 'refreshIntervalSec', label: '대시보드 갱신 주기(초)', baseline: 30, min: 5, max: 300, step: 1, weight: 0.25, impact: { cpu: -0.5, memory: -0.4, storage: -0.1 } },
  ],
  'logging.search': [
    { key: 'logGbPerDay', label: '로그 수집량(GB/일)', baseline: 100, min: 5, max: 10000, step: 5, weight: 0.5, impact: { cpu: 0.6, memory: 0.7, storage: 1 } },
    { key: 'retentionDays', label: '로그 보관 기간(일)', baseline: 30, min: 1, max: 365, step: 1, weight: 0.3, impact: { cpu: 0, memory: 0.2, storage: 1 } },
    { key: 'queryUsers', label: '로그 조회 사용자 수', baseline: 20, min: 1, max: 1000, step: 1, weight: 0.2, impact: { cpu: 0.7, memory: 0.6, storage: 0.2 } },
  ],
  'logging.traceLayer': [
    { key: 'traceSpansPerMin', label: 'Trace Span 수/분', baseline: 50000, min: 1000, max: 3000000, step: 1000, weight: 0.5, impact: { cpu: 0.8, memory: 0.7, storage: 0.5 } },
    { key: 'serviceCount', label: '추적 대상 서비스 수', baseline: 40, min: 5, max: 2000, step: 1, weight: 0.3, impact: { cpu: 0.4, memory: 0.5, storage: 0.3 } },
    { key: 'traceRetentionDays', label: '트레이스 보관 기간(일)', baseline: 7, min: 1, max: 90, step: 1, weight: 0.2, impact: { cpu: 0, memory: 0.2, storage: 1 } },
  ],
  'logging.traceExporter': [
    { key: 'traceSpansPerMin', label: 'Trace Span 수/분', baseline: 50000, min: 1000, max: 3000000, step: 1000, weight: 0.6, impact: { cpu: 0.9, memory: 0.7, storage: 0.2 } },
    { key: 'serviceCount', label: '추적 대상 서비스 수', baseline: 40, min: 5, max: 2000, step: 1, weight: 0.4, impact: { cpu: 0.5, memory: 0.4, storage: 0.1 } },
  ],
}

export function round2(value: number): number {
  return Number(value.toFixed(2))
}

export function ceil2(value: number): number {
  return Math.ceil(value * 100) / 100
}

export function convertGiToUnit(valueGi: number, unit: ResourceUnit): number {
  if (unit === 'Gi') {
    return ceil2(valueGi)
  }
  return ceil2(valueGi * 1024)
}

export function convertUnitToGi(value: number, unit: ResourceUnit): number {
  if (unit === 'Gi') {
    return ceil2(value)
  }
  return ceil2(value / 1024)
}

export function profileFactorByOption(profile: PlanningProfile, optionKey: string): number {
  if (profile === 'standard') {
    return 1
  }

  const isRetention = optionKey.toLowerCase().includes('retention')
  const isInterval = optionKey.toLowerCase().includes('interval')
  const isConcurrency = optionKey === 'concurrentRunners'
  const isThroughput = /(calls|events|pulls|pushes|ops|deployments|commits|targets|spans|query|users|count)/i.test(optionKey)

  if (profile === 'local') {
    if (isRetention) return 0.2
    if (isInterval) return 2.5
    if (isConcurrency) return 0.25
    if (isThroughput) return 0.25
    return 0.3
  }

  if (profile === 'startup') {
    if (isRetention) return 0.45
    if (isInterval) return 1.7
    if (isConcurrency) return 0.35
    if (isThroughput) return 0.45
    return 0.55
  }

  if (isRetention) return 1.8
  if (isInterval) return 0.7
  if (isConcurrency) return 1.8
  if (isThroughput) return 1.7
  return 1.45
}

export function profileAdjustedBaseline(profile: PlanningProfile, def: PlanningOptionDefinition): number {
  const factor = profileFactorByOption(profile, def.key)
  const value = def.baseline * factor
  return Math.min(def.max, Math.max(def.min, ceil2(value)))
}

export function calculateMultipliers(profile: PlanningProfile, slot: PlanningSlot, optionValues: Record<string, number>): ResourceMultipliers {
	const defs = PLANNING_OPTION_DEFS[slot]
	const profileDamping: Record<PlanningProfile, number> = {
		local: 0.2,
		startup: 0.35,
		standard: 0.7,
		enterprise: 0.9,
	}
	const damping = profileDamping[profile]
	const weighted = defs.reduce(
		(sum, def) => {
		const value = optionValues[def.key] ?? def.baseline
			const delta = (value - def.baseline) / def.baseline
			return {
				cpu: sum.cpu + delta * def.weight * def.impact.cpu * damping,
				memory: sum.memory + delta * def.weight * def.impact.memory * damping,
				storage: sum.storage + delta * def.weight * def.impact.storage * damping,
			}
		},
		{ cpu: 0, memory: 0, storage: 0 }
	)

	const profileClampMax: Record<PlanningProfile, { cicd: { cpu: number; memory: number; storage: number }; default: { cpu: number; memory: number; storage: number } }> = {
		local: {
			cicd: { cpu: 1.35, memory: 1.35, storage: 1.25 },
			default: { cpu: 1.25, memory: 1.25, storage: 1.25 },
		},
		startup: {
			cicd: { cpu: 1.9, memory: 1.9, storage: 1.6 },
			default: { cpu: 1.6, memory: 1.6, storage: 1.6 },
		},
		standard: {
			cicd: { cpu: 3.2, memory: 3.2, storage: 2.4 },
			default: { cpu: 2.4, memory: 2.4, storage: 2.2 },
		},
		enterprise: {
			cicd: { cpu: 4.5, memory: 4.5, storage: 3.2 },
			default: { cpu: 3.2, memory: 3.2, storage: 2.8 },
		},
	}

	const clampMax = slot === 'pipeline.cicdPlatform'
		? profileClampMax[profile].cicd
		: profileClampMax[profile].default

  const clamp = (value: number, max: number) => Math.min(max, Math.max(0.5, value))
  const profileResourceScale: Record<PlanningProfile, number> = {
    local: 0.65,
    startup: 0.8,
    standard: 1,
    enterprise: 1.15,
  }
  let rawCpu = 1 + weighted.cpu
  let rawMemory = 1 + weighted.memory
  let rawStorage = 1 + weighted.storage

	if (slot === 'pipeline.cicdPlatform') {
		const runnerDef = defs.find((def) => def.key === 'concurrentRunners')
		if (runnerDef) {
			const runners = optionValues.concurrentRunners ?? runnerDef.baseline
			const ratio = Math.max(0.25, runners / runnerDef.baseline)
			const cpuExpByProfile: Record<PlanningProfile, number> = {
				local: 0.15,
				startup: 0.2,
				standard: 0.35,
				enterprise: 0.45,
			}
			const memExpByProfile: Record<PlanningProfile, number> = {
				local: 0.12,
				startup: 0.18,
				standard: 0.3,
				enterprise: 0.4,
			}
			const cpuBoostCapByProfile: Record<PlanningProfile, number> = {
				local: 1.08,
				startup: 1.15,
				standard: 1.45,
				enterprise: 1.75,
			}
			const memBoostCapByProfile: Record<PlanningProfile, number> = {
				local: 1.06,
				startup: 1.12,
				standard: 1.4,
				enterprise: 1.65,
			}

			const runnerCpuBoost = Math.min(cpuBoostCapByProfile[profile], Math.pow(ratio, cpuExpByProfile[profile]))
			const runnerMemoryBoost = Math.min(memBoostCapByProfile[profile], Math.pow(ratio, memExpByProfile[profile]))

			rawCpu *= runnerCpuBoost
			rawMemory *= runnerMemoryBoost
		}
	}

  const profileScale = profileResourceScale[profile]
  rawCpu *= profileScale
  rawMemory *= profileScale
  rawStorage *= profileScale

  return {
    cpu: clamp(rawCpu, clampMax.cpu),
    memory: clamp(rawMemory, clampMax.memory),
    storage: clamp(rawStorage, clampMax.storage),
    raw: {
      cpu: rawCpu,
      memory: rawMemory,
      storage: rawStorage,
    },
    clamped: {
      cpu: rawCpu !== clamp(rawCpu, clampMax.cpu),
      memory: rawMemory !== clamp(rawMemory, clampMax.memory),
      storage: rawStorage !== clamp(rawStorage, clampMax.storage),
    },
  }
}

export function applyMultipliers(base: {
  cpu_request: number
  cpu_limit: number
  memory_request_gi: number
  memory_limit_gi: number
  storage_request_gi: number
  storage_limit_gi: number
}, multipliers: Pick<ResourceMultipliers, 'cpu' | 'memory' | 'storage'>): ResourceVector {
  return {
    cpuRequest: round2(base.cpu_request * multipliers.cpu),
    cpuLimit: round2(base.cpu_limit * multipliers.cpu),
    memoryRequestGi: round2(base.memory_request_gi * multipliers.memory),
    memoryLimitGi: round2(base.memory_limit_gi * multipliers.memory),
    storageRequestGi: round2(base.storage_request_gi * multipliers.storage),
    storageLimitGi: round2(base.storage_limit_gi * multipliers.storage),
  }
}

export function buildFormulaTooltip(toolLabelValue: string, defs: PlanningOptionDefinition[]): string {
	const clampText = '최종 배수는 최소 0.5배이며, 상한은 프로파일/슬롯별로 보수적으로 적용됩니다.'
  const lines = [
    `${toolLabelValue} 리소스 산정 가이드`,
    '',
    '1) 기본값에서 얼마나 바뀌었는지 계산합니다.',
    '   변화율(Δ) = (입력값 - 기본값) / 기본값',
    '',
    '2) 각 옵션의 영향도를 CPU/Memory/Storage에 따로 반영합니다.',
    '   - w: 옵션 중요도(가중치)',
    '   - a: CPU 영향도',
    '   - m: Memory 영향도',
    '   - s: Storage 영향도',
    '   - 값이 클수록 해당 자원에 더 크게 반영됩니다.',
    '   - 음수면(예: interval) 값이 커질수록 부하가 줄어듭니다.',
    '',
    '3) 추천값 계산식',
    '   CPU 추천 = 기본 CPU × (1 + Σ(w × a × Δ))',
    '   MEM 추천 = 기본 MEM × (1 + Σ(w × m × Δ))',
    '   STO 추천 = 기본 STO × (1 + Σ(w × s × Δ))',
    `   ${clampText}`,
    '',
    '4) 적용값',
    '   - 처음에는 추천값으로 자동 세팅됩니다.',
    '   - 이후 직접 수정할 수 있습니다.',
    '   - 플래닝 옵션을 다시 바꾸면 추천값 기준으로 재설정됩니다.',
    '',
    '옵션별 계수:',
  ]

  defs.forEach((def) => {
    lines.push(`${def.label}: w=${def.weight}, a=${def.impact.cpu}, m=${def.impact.memory}, s=${def.impact.storage}`)
  })

  if (defs.some((def) => def.key === 'concurrentRunners')) {
    lines.push('')
    lines.push('추가 규칙(CI/CD): 동시 러너 수는 CPU/MEM에 배수 계수로 추가 반영됩니다.')
	lines.push('CPU 추가 배수 = min(cap, (동시러너 / 기준러너)^exp)')
	lines.push('MEM 추가 배수 = min(cap, (동시러너 / 기준러너)^exp)')
	}

  return lines.join('\n')
}
