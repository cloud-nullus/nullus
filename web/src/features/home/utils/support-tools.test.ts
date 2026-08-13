import { existsSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  ARTIFACTS_OPTIONS,
  AUTHENTICATION_OPTIONS,
  LOGGING_OPTIONS,
  MONITORING_OPTIONS,
  PIPELINE_OPTIONS,
} from '../../stack/utils/install-constants'
import en from '../../../i18n/en.json'
import ko from '../../../i18n/ko.json'
import { SUPPORT_TOOLS, supportToolIconURL } from './support-tools'

/** 설치 마법사가 실제로 고르게 하는 OSS 선택지 전부. */
function wizardOptionIDs(): string[] {
  const groups = [ARTIFACTS_OPTIONS, PIPELINE_OPTIONS, MONITORING_OPTIONS, LOGGING_OPTIONS]
  const ids = groups.flatMap((group) => Object.values(group).flat()).map((option) => option.id)
  return [...new Set([...ids, ...AUTHENTICATION_OPTIONS.map((option) => option.id)])]
}

const COVERED = new Set(SUPPORT_TOOLS.flatMap((tool) => tool.wizardIds))

/**
 * 마법사에는 있지만 메인에서 "지원한다" 고 말하지 않는 선택지와, 그렇게 정한 이유.
 *
 * 전부 백엔드에 배선이 없다 — helm 단계 카탈로그(`helm_step_metadata.go`)에도,
 * 차트 분기(`helm-values.go` 의 `resolveChartSpecForStep`)에도, 외부 연동
 * 어댑터에도 나오지 않는다. 마법사가 고르게는 하지만 배포해도 아무것도
 * 설치되지 않으므로, 메인에 걸면 "지원한다" 가 거짓이 된다.
 *
 * 배선이 생기면 여기서 빼고 카드를 추가한다 — 반대로 하면 아래 테스트가 막는다.
 */
const NOT_INSTALLABLE: Record<string, string> = {
  gitea: '설치 단계 없음',
  jenkins: '설치 단계 없음',
  flux: '설치 단계 없음',
  spinnaker: '설치 단계 없음',
  jfrog: '설치 단계 없음',
  'docker-hub': '설치 단계 없음 (FE 의 차트 메타는 실재하지 않는 이름이다)',
  thanos: '설치 단계 없음',
  victoriametrics: '설치 단계 없음',
  'opensearch-dashboards': '설치 단계 없음 — OpenSearch 본체만 설치된다',
}

describe('지원 OSS 목록', () => {
  // 마법사의 선택지는 이 목록보다 넓다. 넓다는 사실 자체는 문제가 아니지만,
  // 어느 쪽인지 정하지 않은 선택지가 생기는 것은 문제다 — 새 OSS 가 메인에서
  // 조용히 빠지거나, 배선 없는 도구가 조용히 "지원" 으로 붙는다. 둘 중 하나를
  // 반드시 명시하게 만든다.
  it('마법사의 모든 선택지가 카드이거나 배선 없음으로 선언되어 있다', () => {
    const undecided = wizardOptionIDs().filter((id) => !COVERED.has(id) && !(id in NOT_INSTALLABLE))
    expect(
      undecided,
      `카드로 올리거나 NOT_INSTALLABLE 에 적어야 하는 마법사 선택지:\n${undecided.join('\n')}`,
    ).toEqual([])
  })

  // 같은 도구가 양쪽에 있으면 목록이 스스로 모순된다.
  it('배선 없다고 선언한 도구를 지원한다고 적지 않는다', () => {
    const contradictions = SUPPORT_TOOLS.flatMap((tool) => tool.wizardIds).filter((id) => id in NOT_INSTALLABLE)
    expect(contradictions, `NOT_INSTALLABLE 인데 카드에 있는 id:\n${contradictions.join('\n')}`).toEqual([])
  })

  // 마법사에서 사라진 도구가 목록에 남으면 "지원한다" 가 거짓이 된다.
  // MinIO 는 사용자가 고르는 것이 아니라 고정 설치라 예외다.
  it('마법사에 없는 도구를 지원한다고 적지 않는다', () => {
    const wizard = new Set([...wizardOptionIDs(), 'minio'])
    const orphans = SUPPORT_TOOLS.flatMap((tool) => tool.wizardIds).filter((id) => !wizard.has(id))
    expect(orphans, `마법사에 없는 id:\n${orphans.join('\n')}`).toEqual([])
  })

  // NOT_INSTALLABLE 은 마법사를 향한 목록이다. 마법사에서 선택지가 사라졌는데
  // 여기 남아 있으면, 다음 사람이 "왜 빠져 있지" 하고 근거 없는 항목을 붙든다.
  it('배선 없음 선언이 마법사에 실재하는 선택지만 가리킨다', () => {
    const wizard = new Set(wizardOptionIDs())
    const stale = Object.keys(NOT_INSTALLABLE).filter((id) => !wizard.has(id))
    expect(stale, `마법사에서 사라진 NOT_INSTALLABLE 항목:\n${stale.join('\n')}`).toEqual([])
  })

  it('제품 이름이 중복되지 않는다', () => {
    const names = SUPPORT_TOOLS.map((tool) => tool.name)
    expect(names).toEqual([...new Set(names)])
  })

  // 파일이 없으면 화면에는 깨진 이미지가 뜬다. 아이콘 이름 오타와
  // 자산 삭제를 배포 전에 잡는다.
  it('모든 아이콘 파일이 실제로 있다', () => {
    const missing = SUPPORT_TOOLS.filter(
      (tool) => !existsSync(join(__dirname, '../../../../public', supportToolIconURL(tool.icon))),
    ).map((tool) => `${tool.name} → ${supportToolIconURL(tool.icon)}`)
    expect(missing, `없는 아이콘 파일:\n${missing.join('\n')}`).toEqual([])
  })

  it('카테고리 라벨 키가 en/ko 양쪽에 있다', () => {
    const resolve = (source: unknown, key: string) =>
      key.split('.').reduce<unknown>(
        (cur, part) => (cur !== null && typeof cur === 'object' ? (cur as Record<string, unknown>)[part] : undefined),
        source,
      )
    const missing = SUPPORT_TOOLS.map((tool) => tool.categoryKey)
      .filter((key, index, all) => all.indexOf(key) === index)
      .filter((key) => typeof resolve(en, key) !== 'string' || typeof resolve(ko, key) !== 'string')
    expect(missing, `번역이 없는 카테고리 키:\n${missing.join('\n')}`).toEqual([])
  })
})
