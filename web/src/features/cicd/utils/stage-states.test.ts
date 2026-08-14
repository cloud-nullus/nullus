import { describe, it, expect } from 'vitest'
import { buildStageStates } from './stage-states'

describe('buildStageStates', () => {
  // 배포 상태 하나로 모든 단계를 칠하면 돌지도 않은 단계가 성공으로 보인다.
  // 실제로 Jenkinsfile 은 Build·Deploy 2단계인데 템플릿의 4단계가 모두
  // "Completed" 로 그려졌고, 같은 화면이 "0 steps" 라고 스스로 밝히고 있었다.
  it('스텝 정보가 없으면 성공으로 칠하지 않는다', () => {
    const states = buildStageStates(['Build', 'Test', 'ImageBuild', 'Deploy'], [])
    expect(states).toEqual(['unknown', 'unknown', 'unknown', 'unknown'])
  })

  it('실제 스텝 결과를 단계에 맞춘다', () => {
    const states = buildStageStates(
      ['Build', 'Deploy'],
      [
        { name: 'Build', status: 'success' },
        { name: 'Deploy', status: 'running' },
      ],
    )
    expect(states).toEqual(['completed', 'in_progress'])
  })

  // 템플릿에는 있지만 실행되지 않은 단계는 모른다고 말해야 한다.
  it('보고되지 않은 단계는 unknown 이다', () => {
    const states = buildStageStates(
      ['Build', 'Test', 'Deploy'],
      [{ name: 'Build', status: 'success' }],
    )
    expect(states).toEqual(['completed', 'unknown', 'unknown'])
  })

  it('실패한 스텝은 실패로 표시한다', () => {
    expect(buildStageStates(['Build'], [{ name: 'Build', status: 'failed' }])).toEqual(['failed'])
  })

  it('단계가 없으면 빈 배열이다', () => {
    expect(buildStageStates([], [{ name: 'Build', status: 'success' }])).toEqual([])
  })
})
