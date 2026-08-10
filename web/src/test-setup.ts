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

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: unknown) => {
      const resolved = resolveKey(en as unknown as Record<string, unknown>, key)
      return options !== null && typeof options === 'object'
        ? interpolate(resolved, options as Record<string, unknown>)
        : resolved
    },
    i18n: { changeLanguage: vi.fn(), language: 'en' },
  }),
  I18nextProvider: ({ children }: { children: React.ReactNode }) => children,
  initReactI18next: { type: '3rdParty', init: vi.fn() },
}))
