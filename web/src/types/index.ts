export type Role = 'admin' | 'devops' | 'developer'

export type Theme = 'dark' | 'light'

export type ClusterStatus = 'connected' | 'pending' | 'error' | 'inactive'

export type DeploymentState = 'running' | 'success' | 'failed' | 'pending' | 'cancelled'

export interface User {
  id: string
  name: string
  email: string
  role: Role
}
