import { describe, it, expect } from 'vitest'
import { getCompatIssueMessage, COMPAT_ISSUE_I18N } from './compat-issue-i18n'

describe('getCompatIssueMessage', () => {
  it('resolves a known code to its i18n key (translator wins over fallback)', () => {
    const t = (key: string, fallback?: string) =>
      key === COMPAT_ISSUE_I18N.TOOL_ARCH_UNSUPPORTED
        ? 'localized arch message'
        : (fallback ?? '')
    const out = getCompatIssueMessage(t, {
      code: 'TOOL_ARCH_UNSUPPORTED',
      message: 'original arch message',
    })
    expect(out).toBe('localized arch message')
  })

  it('returns the raw message when the code is unknown', () => {
    const t = (_key: string, fallback?: string) => fallback ?? ''
    const out = getCompatIssueMessage(t, {
      code: 'SOMETHING_NEW',
      message: 'fallback to server text',
    })
    expect(out).toBe('fallback to server text')
  })

  it('returns the raw message when no code is present', () => {
    const t = (_key: string, fallback?: string) => fallback ?? ''
    const out = getCompatIssueMessage(t, { message: 'no code here' })
    expect(out).toBe('no code here')
  })
})
