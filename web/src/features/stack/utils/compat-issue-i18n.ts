// F8-UIUX-ServerVerdictI18n — map the backend compatibility issue codes
// that land in serverVerdict.issues (from /stacks/:id/validate and the
// DEPLOY_COMPAT_FAIL / DEPLOY_COMPAT_WARN_UNACK response bodies) to user-
// facing i18n keys. Keeps the raw code out of the visible UI while still
// keeping it queryable via the `data-code` attribute for E2E/debug.

export const COMPAT_ISSUE_I18N: Record<string, string> = {
  TOOL_ARCH_UNSUPPORTED: 'stackInstall.compatibility.issue.toolArchUnsupported',
  CLUSTER_ARCH_UNKNOWN: 'stackInstall.compatibility.issue.clusterArchUnknown',
  KUBECONFIG_NOT_REGISTERED: 'stackInstall.compatibility.issue.kubeconfigNotRegistered',
  DEPLOY_COMPAT_FAIL: 'stackInstall.compatibility.issue.serverFail',
  DEPLOY_COMPAT_WARN_UNACK: 'stackInstall.compatibility.issue.serverWarnAck',
}

interface CompatIssueLike {
  code?: string
  message?: string
}

// Matches the same (key, defaultValue?) shape already used by
// getStackStatusLabel in stack-list-page.tsx. react-i18next's TFunction
// surfaces a pre-existing type-only mismatch on this shape, but it works
// at runtime and keeps the helper API uniform with the rest of the feature.
type Translator = (key: string, defaultValue?: string) => string

export function getCompatIssueMessage(t: Translator, issue: CompatIssueLike): string {
  const fallback = issue.message ?? ''
  if (!issue.code) return fallback
  const key = COMPAT_ISSUE_I18N[issue.code]
  if (!key) return fallback
  return t(key, fallback)
}
