import type { CompatibilityMatrix, CompatibilityTool } from '../../../types'

// CompatibilityArchVerdict mirrors the three-state verdict the Pre-Deploy
// Gate renders: the cluster is confirmed safe, the cluster is confirmed
// incompatible, or the cluster architecture is not yet known.
export type CompatibilityArchVerdict = 'compatible' | 'incompatible' | 'unknown'

// toolSupportsArch matches the backend's ToolVersion.SupportsArch: an empty
// ArchSupport slice is treated as "amd64 only" for backward compatibility
// with v1 matrices that predate F8 Task 1.
export function toolSupportsArch(tool: CompatibilityTool, arch: string): boolean {
  if (!arch) {
    return false
  }
  if (!tool.archSupport || tool.archSupport.length === 0) {
    return arch === 'amd64'
  }
  return tool.archSupport.includes(arch)
}

// isMatrixCompatibleWithCluster reports whether every tool in the matrix
// ships images for every architecture in the cluster's node fleet.
//
//   - empty/unknown cluster archs    => 'unknown'  (caller should prompt
//                                       the user to Refresh Discovery)
//   - every tool covers every arch   => 'compatible'
//   - any tool misses any arch       => 'incompatible'
export function isMatrixCompatibleWithCluster(
  matrix: CompatibilityMatrix,
  clusterArchs: string[] | undefined,
): CompatibilityArchVerdict {
  if (!clusterArchs || clusterArchs.length === 0) {
    return 'unknown'
  }
  for (const tool of matrix.tools) {
    for (const arch of clusterArchs) {
      if (!toolSupportsArch(tool, arch)) {
        return 'incompatible'
      }
    }
  }
  return 'compatible'
}

// matrixArchMismatches enumerates which (tool, arch) pairs break compatibility.
export interface MatrixArchMismatch {
  toolName: string
  missingArchs: string[]
}

export function matrixArchMismatches(
  matrix: CompatibilityMatrix,
  clusterArchs: string[] | undefined,
): MatrixArchMismatch[] {
  if (!clusterArchs || clusterArchs.length === 0) {
    return []
  }
  const mismatches: MatrixArchMismatch[] = []
  for (const tool of matrix.tools) {
    const missing = clusterArchs.filter((arch) => !toolSupportsArch(tool, arch))
    if (missing.length > 0) {
      mismatches.push({ toolName: tool.name, missingArchs: missing })
    }
  }
  return mismatches
}
