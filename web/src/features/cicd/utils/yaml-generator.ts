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

export function generateYaml(form: YamlFormValues, appType: string): string {
  const cpu = form.cpuLimit ?? '500m'
  const mem = form.memoryLimit ?? '512Mi'
  const replicas = form.replicas ?? 2
  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${form.appName ?? 'my-app'}
  namespace: ${form.namespace ?? 'default'}
  labels:
    app: ${form.appName ?? 'my-app'}
    template: ${appType || 'backend'}
spec:
  replicas: ${replicas}
  selector:
    matchLabels:
      app: ${form.appName ?? 'my-app'}
  template:
    metadata:
      labels:
        app: ${form.appName ?? 'my-app'}
    spec:
      containers:
        - name: ${form.appName ?? 'my-app'}
          image: harbor.nullus.io/${form.appName ?? 'my-app'}:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              cpu: ${form.cpuRequest ?? '100m'}
              memory: ${form.memoryRequest ?? '128Mi'}
            limits:
              cpu: ${cpu}
              memory: ${mem}
${(form.envVars ?? []).filter((e) => e.key).length > 0
  ? `          env:\n${(form.envVars ?? []).filter((e) => e.key).map((e) => `            - name: ${e.key}\n              value: "${e.value}"`).join('\n')}`
  : ''}
---
apiVersion: v1
kind: Service
metadata:
  name: ${form.appName ?? 'my-app'}-svc
  namespace: ${form.namespace ?? 'default'}
spec:
  selector:
    app: ${form.appName ?? 'my-app'}
  ports:
    - port: 80
      targetPort: 8080`
}
