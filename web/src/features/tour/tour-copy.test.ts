import { describe, expect, it } from 'vitest'
import en from '../../i18n/en.json'
import ko from '../../i18n/ko.json'
import { TOUR_STEPS } from './tour-steps'

type Copy = Record<string, { title?: string; body?: string } | undefined>

// 문구가 없는 걸음은 화면에 걸음 id 가 그대로 뜬다("installDryRun"). 걸음을
// 추가하고 번역을 잊는 실수를 여기서 잡는다.
describe('투어 문구', () => {
  for (const [language, bundle] of [['en', en], ['ko', ko]] as const) {
    it(`${language} — 모든 걸음에 제목과 설명이 있다`, () => {
      const steps = (bundle as { tour: { steps: Copy } }).tour.steps
      const missing = TOUR_STEPS.filter((step) => !steps[step.id]?.title || !steps[step.id]?.body)
      expect(missing.map((step) => step.id)).toEqual([])
    })
  }

  it('걸음 id 는 중복되지 않는다', () => {
    const ids = TOUR_STEPS.map((step) => step.id)
    expect(new Set(ids).size).toBe(ids.length)
  })
})
