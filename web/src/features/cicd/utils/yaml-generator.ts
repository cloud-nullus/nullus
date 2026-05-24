interface YamlFormValues {
  appName?: string
  namespace?: string
  replicas?: number
  cpuRequest?: string
  cpuLimit?: string
  memoryRequest?: string
  memoryLimit?: string
  envVars?: Array<{ key: string; value: string }>
}

function yamlSafe(value: string): string {
  if (/[:\n\r#"'\\{}\[\],&*?|><!%@`]/.test(value) || value !== value.trim()) {
    return `"${value.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, '\\n')}"`
  }
  return value
}

export function generateYaml(form: YamlFormValues, appType: string): string {
  const name = yamlSafe(form.appName ?? 'my-app')
  const namespace = yamlSafe(form.namespace ?? 'default')
  const template = yamlSafe(appType || 'backend')
  const cpu = form.cpuLimit ?? '500m'
  const mem = form.memoryLimit ?? '512Mi'
  const replicas = form.replicas ?? 2
  const envLines = (form.envVars ?? [])
    .filter((e) => e.key)
    .map((e) => `            - name: ${yamlSafe(e.key)}\n              value: ${yamlSafe(e.value)}`)
    .join('\n')

  return `apiVersion: apps/v1
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
          image: harbor.nullus.io/${name}:latest
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
---
apiVersion: v1
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
}
