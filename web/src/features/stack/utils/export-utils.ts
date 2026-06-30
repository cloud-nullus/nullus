export type StackExportFormat = 'json' | 'yaml'

const DEFAULT_EXPORT_BASE = 'stack-export'

export function buildStackExportBaseName(stackName: string): string {
  const normalized = stackName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')

  return normalized || DEFAULT_EXPORT_BASE
}

export function buildStackExportFilename(
  stackName: string,
  format: StackExportFormat,
): string {
  return `${buildStackExportBaseName(stackName)}.${format}`
}

export function normalizeExportFilename(
  filename: string,
  format: StackExportFormat,
  fallbackBase = DEFAULT_EXPORT_BASE,
): string {
  const trimmed = filename.trim()
  const base = trimmed ? trimmed.replace(/\.[^.]+$/, '') : fallbackBase
  return `${base || fallbackBase}.${format}`
}

export function parseContentDispositionFilename(
  header?: string | null,
): string | null {
  if (!header) return null

  const starMatch = header.match(/filename\*\s*=\s*([^;]+)/i)
  if (starMatch?.[1]) {
    const raw = starMatch[1].trim().replace(/^UTF-8''/i, '').replace(/^"|"$/g, '')
    try {
      return decodeURIComponent(raw)
    } catch {
      return raw
    }
  }

  const match = header.match(/filename\s*=\s*(?:"([^"]+)"|([^;]+))/i)
  const value = match?.[1] ?? match?.[2]
  return value?.trim() || null
}

export function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.rel = 'noreferrer'
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}
