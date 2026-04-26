export type LocaleLike = string | null | undefined
export type DateInput = string | number | Date | null | undefined

const DEFAULT_LOCALE = 'en-US'
const KOREAN_LOCALE = 'ko-KR'

const DEFAULT_DATE_OPTIONS: Intl.DateTimeFormatOptions = {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
}

const DEFAULT_TIME_OPTIONS: Intl.DateTimeFormatOptions = {
  hour: '2-digit',
  minute: '2-digit',
}

const DEFAULT_DATE_TIME_OPTIONS: Intl.DateTimeFormatOptions = {
  ...DEFAULT_DATE_OPTIONS,
  ...DEFAULT_TIME_OPTIONS,
}

function toDate(value: DateInput): Date | null {
  if (!value) return null
  const parsed = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(parsed.getTime())) return null
  return parsed
}

export function resolveLocale(locale: LocaleLike): string {
  if (!locale) return DEFAULT_LOCALE
  return locale.toLowerCase().startsWith('ko') ? KOREAN_LOCALE : DEFAULT_LOCALE
}

export function formatDate(
  value: DateInput,
  locale: LocaleLike,
  options: Intl.DateTimeFormatOptions = DEFAULT_DATE_OPTIONS
): string {
  const date = toDate(value)
  if (!date) return '-'
  return date.toLocaleDateString(resolveLocale(locale), options)
}

export function formatTime(
  value: DateInput,
  locale: LocaleLike,
  options: Intl.DateTimeFormatOptions = DEFAULT_TIME_OPTIONS
): string {
  const date = toDate(value)
  if (!date) return '-'
  return date.toLocaleTimeString(resolveLocale(locale), options)
}

export function formatDateTime(
  value: DateInput,
  locale: LocaleLike,
  options: Intl.DateTimeFormatOptions = DEFAULT_DATE_TIME_OPTIONS
): string {
  const date = toDate(value)
  if (!date) return '-'
  return date.toLocaleString(resolveLocale(locale), options)
}
