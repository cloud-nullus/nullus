// F8-Phase3 follow-up: pure policy describing which stack states are
// eligible for retry. Mirrors the backend contract in
// internal/stack/adapter/handler/deploy_handler.go `Retry` handler which
// accepts only `failed` and `rolled_back` and otherwise returns
// STACK_RETRY_INVALID_STATE (409).

export type StackStatus =
  | 'pending'
  | 'validating'
  | 'installing'
  | 'configuring'
  | 'health_check'
  | 'completed'
  | 'failed'
  | 'rolling_back'
  | 'rolled_back'
  | 'cancelled'

export function canRetry(status: StackStatus): boolean {
  return status === 'failed' || status === 'rolled_back'
}
