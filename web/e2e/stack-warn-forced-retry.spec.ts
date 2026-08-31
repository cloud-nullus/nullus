import { test, expect } from '@playwright/test'
import { loginAs } from './helpers/auth'

// F8 Task 7 — warn-forced Retry/Rollback UI smoke.
//
// Scope: the spec validates the *UI contract* for the warn-ack path:
// the admin Stack Version Management page surfaces the untested
// warn-prone matrix with the `untested` status badge. Driving a full
// deploy-to-terminal-state flow
// requires a live Kind cluster (tracked separately under F8 Task 6 and
// F8-F6-Cloud); here we only assert what the UI exposes so downstream
// operators can complete the flow.
//
// Tagged @stack-critical so it's picked up by `pnpm e2e:stack-critical`.

test.describe('F8 Task 7 — Warn-Forced Retry/Rollback UI', () => {
  test('admin page surfaces the untested matrix (warn-prone) @stack-critical', async ({ page }) => {
    await loginAs(page, 'admin')

    // 1. Admin Stack Version Management page shows the github-argocd-v1
    //    matrix with the `untested` status badge — the same matrix that
    //    the Go e2e suite uses to trigger the warn verdict (TOOL_ARCH_UNSUPPORTED
    //    against an arm64-only cluster).
    await page.goto('/admin/stack-versions')
    await expect(page.getByRole('heading', { name: /Stack Version Management/i })).toBeVisible()
    await expect(page.getByText('github-argocd-v1').first()).toBeVisible()

    // Click the untested matrix to open the detail panel.
    await page.getByRole('button', { name: /GitHub \+ Argo CD/i }).click()
    // Detail panel header repeats the status with an `untested` badge — the
    // ko/en i18n variants both keep `untested` as the lowercase key.
    await expect(page.getByText(/untested|미검증/i).first()).toBeVisible()

  })
})
