export type StageState =
  | 'queued'
  | 'in_progress'
  | 'completed'
  | 'failed'
  | 'unknown'

/**
 * buildStageStates 는 실제 실행된 스텝에서 단계 상태를 만든다.
 *
 * 배포 상태 하나로 모든 단계를 칠하지 않는다. 그렇게 하면 돌지도 않은 단계가
 * "Completed" 로 보인다 — 이 파이프라인의 Jenkinsfile 은 Build·Deploy 2단계인데
 * 템플릿에 적힌 4단계가 모두 성공한 것처럼 그려졌다. 화면이 실행되지 않은 일을
 * 성공했다고 말하면 안 된다.
 *
 * 스텝 정보가 없으면 'unknown' 이다 — 초록 체크 대신 모른다고 말한다.
 */
export function buildStageStates(
  stages: string[],
  steps: { name?: string; status?: string }[],
): StageState[] {
  if (stages.length === 0) return []
  if (steps.length === 0) return stages.map(() => 'unknown')

  const byName = new Map<string, string>()
  for (const step of steps) {
    if (step.name) byName.set(step.name.toLowerCase(), (step.status ?? '').toLowerCase())
  }

  return stages.map((stage) => {
    const status = byName.get(stage.toLowerCase())
    if (status === undefined) return 'unknown'
    if (status === 'success' || status === 'completed') return 'completed'
    if (status === 'failed' || status === 'error') return 'failed'
    if (status === 'running' || status === 'in_progress') return 'in_progress'
    return 'queued'
  })
}

/**
 * resolvePipelineStages 는 이력 화면에 그릴 단계 목록을 고른다.
 *
 * 이미지 스캔 같은 선택 단계는 파이프라인마다 켜고 끈다. 템플릿은 그것을 알 수
 * 없으므로, 파이프라인이 기록한 단계가 있으면 그것을 믿는다 — 템플릿을 쓰면
 * 돌고 있는 스캔 단계가 화면에서 사라진다.
 *
 * 기록이 비어 있으면 템플릿으로 떨어진다. 단계 기록이 생기기 전에 만든
 * 파이프라인이 그렇고, 비었다고 단계가 없는 것으로 그리면 이력이 통째로 빈다.
 */
export function resolvePipelineStages(
  pipelineStages: string[] | undefined,
  templateStages: string[] | undefined,
): string[] {
  if (pipelineStages && pipelineStages.length > 0) return pipelineStages
  return templateStages ?? []
}
