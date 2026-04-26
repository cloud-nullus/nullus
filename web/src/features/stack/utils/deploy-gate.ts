// F8-UIUX-DeployGateServerCheck — pure predicate that decides whether the
// Deploy button should be disabled based on the server-side Pre-Deploy Gate
// verdict alone. The wizard's existing client-side compatibility gate is
// combined separately at the call site.

interface ServerVerdictLike {
  overall: { state: 'pass' | 'warn' | 'fail' | string }
}

export function isDeployServerGateLocked(
  verdict: ServerVerdictLike | null | undefined,
  serverWarnAcknowledged: boolean,
): boolean {
  if (!verdict) return false
  if (verdict.overall.state === 'fail') return true
  if (verdict.overall.state === 'warn' && !serverWarnAcknowledged) return true
  return false
}
