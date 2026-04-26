// F8-UIUX-WarnAckPersist — persist compatibility warn-ack toggles across
// page reloads. The Install wizard resets its draft state on refresh, and
// the warn-ack checkbox gets cleared with it, which forces users to re-
// acknowledge exactly the same verdict they just saw seconds earlier.
//
// We write a marker under sessionStorage (not localStorage — persisting
// acknowledgements across browser sessions would be too strong) keyed by
// (kind, stackName, clusterId, verdictHash). verdictHash is a djb2 digest
// of the issue list so replacing one warn with a different warn rotates
// the key and correctly re-arms the gate.

function djb2(input: string): string {
  let h = 5381
  for (let i = 0; i < input.length; i++) {
    h = ((h << 5) + h + input.charCodeAt(i)) | 0
  }
  return (h >>> 0).toString(16).padStart(8, '0')
}

export function warnAckKey(
  kind: 'client' | 'server',
  stackName: string,
  clusterId: string,
  issues: unknown,
): string {
  const name = (stackName || '').trim() || '_'
  const cluster = (clusterId || '').trim() || '_'
  const hash = djb2(JSON.stringify(issues ?? []))
  return `nullus.warnAck.${kind}.${name}.${cluster}.${hash}`
}

export function readAck(key: string): boolean {
  try {
    return sessionStorage.getItem(key) === '1'
  } catch {
    // private-mode / quota-exceeded / disabled storage — silently fall back
    // to the in-memory state; the user can still ack via the checkbox for
    // the current session.
    return false
  }
}

export function writeAck(key: string, ack: boolean): void {
  try {
    if (ack) {
      sessionStorage.setItem(key, '1')
    } else {
      sessionStorage.removeItem(key)
    }
  } catch {
    /* noop — see readAck */
  }
}
