import type { CompatibilityValidationResult } from '../../../types'

// ServerVerdictDecision is the output of shouldBlockOnServerVerdict and tells
// the wizard how to proceed after the server-side Pre-Deploy Gate (F8-F3)
// returns a verdict:
//
//   - pass      : deploy immediately.
//   - warn-ack  : show the ack prompt; deploy only after the user confirms.
//   - block     : surface the fail verdict and do not deploy.
export type ServerVerdictDecision = {
  mode: 'pass' | 'warn-ack' | 'block'
  block: boolean
  // acknowledgeWarnings is the flag the caller should pass to deployStack
  // once the user has confirmed. Always undefined for pass / block.
  acknowledgeWarnings?: boolean
}

// shouldBlockOnServerVerdict is a pure function so the wizard behavior can
// be unit tested without React or MSW. Policy mirrors the server:
//
//   - overall.state == "fail"   => block.
//   - overall.state == "warn"   => require ack; if the caller has already
//                                    collected ack, return warn-ack with
//                                    the flag set so deploy proceeds; if
//                                    no ack yet, return block so the UI
//                                    renders the prompt.
//   - overall.state == "pass"   => pass.
export function shouldBlockOnServerVerdict(
  verdict: CompatibilityValidationResult,
  userAcknowledged: boolean,
): ServerVerdictDecision {
  if (verdict.overall.state === 'fail') {
    return { mode: 'block', block: true }
  }
  if (verdict.overall.state === 'warn') {
    if (userAcknowledged) {
      return { mode: 'warn-ack', block: false, acknowledgeWarnings: true }
    }
    return { mode: 'block', block: true }
  }
  return { mode: 'pass', block: false }
}
