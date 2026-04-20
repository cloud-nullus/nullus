// F8-Phase3 follow-up: shared deploy / retry error extraction.
// Parses the DEPLOY_COMPAT_FAIL / DEPLOY_COMPAT_WARN_UNACK verdict body
// the backend attaches so both the Install Wizard and the Retry UI can
// render the issue list with consistent copy.

export interface DeployCompatError {
  code: 'DEPLOY_COMPAT_FAIL' | 'DEPLOY_COMPAT_WARN_UNACK'
  issueLines: string[]
}

// extractDeployCompatError pulls out the structured verdict from an axios
// (or similar) error response. Returns null when the error is not a
// compatibility-gate rejection so callers can fall through to a generic
// formatter.
export function extractDeployCompatError(error: unknown): DeployCompatError | null {
  if (typeof error !== 'object' || error === null) return null
  const record = error as Record<string, unknown>
  const details = record.details
  if (typeof details !== 'object' || details === null) return null
  const nestedError = (details as Record<string, unknown>).error
  if (typeof nestedError !== 'object' || nestedError === null) return null
  const nested = nestedError as Record<string, unknown>
  const code = typeof nested.code === 'string' ? nested.code : ''
  if (code !== 'DEPLOY_COMPAT_FAIL' && code !== 'DEPLOY_COMPAT_WARN_UNACK') {
    return null
  }
  const verdict = nested.verdict
  const issueLines: string[] = []
  if (typeof verdict === 'object' && verdict !== null) {
    const issues = (verdict as Record<string, unknown>).issues
    if (Array.isArray(issues)) {
      for (const issue of issues) {
        if (typeof issue === 'object' && issue !== null) {
          const rec = issue as Record<string, unknown>
          const iCode = typeof rec.code === 'string' ? rec.code : ''
          const msg = typeof rec.message === 'string' ? rec.message : ''
          if (msg) {
            issueLines.push(iCode ? `[${iCode}] ${msg}` : msg)
          }
        }
      }
    }
  }
  return { code: code as DeployCompatError['code'], issueLines }
}
