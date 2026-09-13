import { describe, it, expect } from 'vitest'
import { buildStageStates, resolvePipelineStages } from './stage-states'

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

  // 취소는 백엔드에서 실패와 따로 보고된다(스캔 판정을 남기지 않기 위해). 화면은
  // 지금처럼 끝나지 못한 단계로 보여 준다 — 대기 중으로 그리면 아직 돌 것처럼 보인다.
  it('취소된 단계는 실패로 표시한다', () => {
    expect(
      buildStageStates(['Build', 'ImageScan'], [
        { name: 'build', status: 'success' },
        { name: 'image-scan', status: 'canceled' },
      ]),
    ).toEqual(['completed', 'failed'])
  })

  // GitLab·GitHub 은 잡 키(image-scan)로 보고하고 파이프라인 단계는 ImageScan 이다.
  // 대소문자만 맞추면 돌고 있는 스캔 단계가 "모름" 으로 그려진다.
  it('CI 마다 다른 단계 이름 표기를 같은 단계로 맞춘다', () => {
    expect(
      buildStageStates(
        ['Build', 'ImageScan', 'Deploy'],
        [
          { name: 'build', status: 'success' },
          { name: 'image-scan', status: 'failed' },
          { name: 'deploy', status: 'running' },
        ],
      ),
    ).toEqual(['completed', 'failed', 'in_progress'])
  })
})

describe('resolvePipelineStages', () => {
  // 이미지 스캔 같은 선택 단계는 파이프라인마다 켜고 끈다. 템플릿은 그것을 알 수
  // 없으므로 파이프라인이 기록한 단계가 있으면 그것을 믿는다.
  it('파이프라인이 기록한 단계가 있으면 그것을 쓴다', () => {
    expect(
      resolvePipelineStages(['Build', 'ImageScan', 'Deploy'], ['Build', 'Deploy']),
    ).toEqual(['Build', 'ImageScan', 'Deploy'])
  })

  // 단계 기록이 생기기 전에 만든 파이프라인은 빈 값이다. 비었다고 단계가 없는
  // 것으로 그리면 이력 화면이 통째로 비어 보인다 — 템플릿으로 떨어진다.
  it('기록이 없으면 템플릿 단계로 떨어진다', () => {
    expect(resolvePipelineStages(undefined, ['Build', 'Deploy'])).toEqual(['Build', 'Deploy'])
    expect(resolvePipelineStages([], ['Build', 'Deploy'])).toEqual(['Build', 'Deploy'])
  })

  it('둘 다 없으면 빈 배열이다', () => {
    expect(resolvePipelineStages(undefined, undefined)).toEqual([])
  })
})
