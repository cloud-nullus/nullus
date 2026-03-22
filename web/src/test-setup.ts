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

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => resolveKey(en as unknown as Record<string, unknown>, key),
    i18n: { changeLanguage: vi.fn(), language: 'en' },
  }),
  I18nextProvider: ({ children }: { children: React.ReactNode }) => children,
  initReactI18next: { type: '3rdParty', init: vi.fn() },
}))
