import '@testing-library/jest-dom'
import en from './i18n/en.json'

// jsdom 28 + Vitest 4 localStorage compatibility fix
let _localStorageData: Record<string, string> = {}
vi.stubGlobal('localStorage', {
  getItem: (key: string): string | null => _localStorageData[key] ?? null,
  setItem: (key: string, value: string) => { _localStorageData[key] = String(value) },
  removeItem: (key: string) => { delete _localStorageData[key] },
  clear: () => { _localStorageData = {} },
  get length() { return Object.keys(_localStorageData).length },
  key: (index: number): string | null => Object.keys(_localStorageData)[index] ?? null,
})

// Resolve a dotted key path against a nested object
function resolveKey(obj: Record<string, unknown>, key: string): string {
  const parts = key.split('.')
  let cur: unknown = obj
  for (const part of parts) {
    if (cur !== null && typeof cur === 'object') {
      cur = (cur as Record<string, unknown>)[part]
    } else {
      return key
    }
  }
  return typeof cur === 'string' ? cur : key
}

// i18next 의 {{var}} 치환을 흉내낸다. 이게 없으면 테스트만 원문 그대로 렌더돼
// 실제 화면과 어긋나고, 보간을 쓰는 문구는 테스트로 검증할 수 없다.
function interpolate(template: string, values: Record<string, unknown>): string {
  return template.replace(/\{\{(\w+)\}\}/g, (match, name: string) =>
    values[name] === undefined ? match : String(values[name])
  )
}

// i18next 는 키를 배열로 받으면 **먼저 존재하는 키**를 쓰고, 어느 것도 없으면
// options.defaultValue 로 떨어진다. 목이 그걸 몰라서 배열을 넘기는 순간
// `key.split is not a function` 으로 화면이 통째로 죽었다 — 실제 동작과 목이
// 갈라지면 테스트는 화면이 아니라 목을 검사하게 된다.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string | string[], options?: unknown) => {
      const candidates = Array.isArray(key) ? key : [key]
      const opts =
        options !== null && typeof options === 'object' ? (options as Record<string, unknown>) : undefined

      let resolved: string | undefined
      for (const candidate of candidates) {
        const value = resolveKey(en as unknown as Record<string, unknown>, candidate)
        // resolveKey 는 못 찾으면 키를 그대로 돌려준다.
        if (value !== candidate) {
          resolved = value
          break
        }
      }
      if (resolved === undefined) {
        resolved =
          typeof opts?.defaultValue === 'string'
            ? opts.defaultValue
            : candidates[candidates.length - 1]
      }

      return opts ? interpolate(resolved, opts) : resolved
    },
    i18n: { changeLanguage: vi.fn(), language: 'en' },
  }),
  I18nextProvider: ({ children }: { children: React.ReactNode }) => children,
  initReactI18next: { type: '3rdParty', init: vi.fn() },
}))
