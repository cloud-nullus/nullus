interface YamlFormValues {
  appName?: string
  namespace?: string
  serviceUrl?: string
  imageRepositoryUrl?: string
  replicas?: number
  cpuRequest?: string
  cpuLimit?: string
  memoryRequest?: string
  memoryLimit?: string
  envVars?: Array<{ key: string; value: string }>
}

export interface ManifestYamls {
  deployment: string
  service: string
  ingress: string
}

function yamlSafe(value: string): string {
  if (/[:\n\r#"'\\{}[\],&*?|><!%@`]/.test(value) || value !== value.trim()) {
    return `"${value.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, '\\n')}"`
  }
  return value
}

function serviceHost(value: string | undefined, fallback: string): string {
  const input = value?.trim()
  if (!input) {
    return fallback
  }
  try {
    return new URL(input.includes('://') ? input : `https://${input}`).hostname || fallback
  } catch {
    return input
  }
}

export function generateManifestYamls(form: YamlFormValues, appType: string): ManifestYamls {
  const name = yamlSafe(form.appName ?? 'my-app')
  const namespace = yamlSafe(form.namespace ?? 'default')
  const template = yamlSafe(appType || 'backend')
  const cpu = form.cpuLimit ?? '500m'
  const mem = form.memoryLimit ?? '512Mi'
  const replicas = form.replicas ?? 2
  const host = yamlSafe(serviceHost(form.serviceUrl, `${form.appName ?? 'my-app'}.internal`))
  const imageRepository = form.imageRepositoryUrl?.trim().replace(/:+$/, '') || `harbor.nullus.io/${form.appName ?? 'my-app'}`
  const image = yamlSafe(`${imageRepository}:latest`)
  const envLines = (form.envVars ?? [])
    .filter((e) => e.key)
    .map((e) => `            - name: ${yamlSafe(e.key)}\n              value: ${yamlSafe(e.value)}`)
    .join('\n')

  const deployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${name}
  namespace: ${namespace}
  labels:
    app: ${name}
    template: ${template}
spec:
  replicas: ${replicas}
  selector:
    matchLabels:
      app: ${name}
  template:
    metadata:
      labels:
        app: ${name}
    spec:
      containers:
        - name: ${name}
          image: ${image}
          ports:
            - containerPort: 8080
          resources:
            requests:
              cpu: ${form.cpuRequest ?? '100m'}
              memory: ${form.memoryRequest ?? '128Mi'}
            limits:
              cpu: ${cpu}
              memory: ${mem}
${envLines ? `          env:\n${envLines}` : ''}
`

  const service = `apiVersion: v1
kind: Service
metadata:
  name: ${name}-svc
  namespace: ${namespace}
spec:
  selector:
    app: ${name}
  ports:
    - port: 80
      targetPort: 8080`

  const ingress = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ${name}-ingress
  namespace: ${namespace}
spec:
  rules:
    - host: ${host}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: ${name}-svc
                port:
                  number: 80`

  return { deployment, service, ingress }
}
