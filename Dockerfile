FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/api ./cmd/api

FROM alpine:3.21
# 스택 오케스트레이터가 차트 설치/매니페스트 적용 시 helm·kubectl CLI 를 exec 하므로
# (internal/stack/adapter/helm: installOCIChartWithHelmCLI, kubectl apply 등) 함께 포함한다.
# airgap 에서 OCI 레지스트리(plain-http)에서 차트를 pull 하려면 helm 3.14+ 필요.
ARG HELM_VERSION=v3.16.4
ARG KUBECTL_VERSION=v1.30.0
# DB 마이그레이션은 API 가 기동 시 스스로 돌리지 않는다(차트의 pre-upgrade 훅 Job 과
# airgap/vm-cluster 런북이 밖에서 돌린다). 그 Job 들은 SQL 이 들어 있는 이 이미지를
# 그대로 띄워 migrate 를 실행하므로, SQL 과 그것을 적용할 CLI 가 같은 이미지에 있어야
# 한다 — deploy/csp/vm-cluster/runbook_csp.sh 의 Job 은 migrate 가 없는 채로 이 이미지를
# 부르고 있었고, 돌리면 "migrate: not found" 로 조용히 끝났다.
ARG MIGRATE_VERSION=v4.18.1
# 백업/복구는 pg_dump·pg_restore 를 exec 한다
# (internal/backup/adapter/postgres). 없으면 백업이 "pg_dump: not found" 로
# 조용히 실패하고, 그 사실은 복구를 시도할 때에야 드러난다.
# 클라이언트는 서버 버전 이상이어야 한다 — 차트가 쓰는 PostgreSQL 은 17 계열이다.
# CI/CD 파이프라인의 첫 단계는 이 컨테이너 안에서 소스를 clone 한다
# (internal/cicd/adapter/docker/builder.go: PrepareImage). git 이 없으면
# exec 가 출력 없이 실패해 배포 로그에 "error:" 만 남는다.
RUN apk add --no-cache ca-certificates tzdata git curl postgresql17-client \
    && ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" \
    && curl -fsSL "https://get.helm.sh/helm-${HELM_VERSION}-linux-${ARCH}.tar.gz" | tar -xz -C /tmp \
    && mv "/tmp/linux-${ARCH}/helm" /usr/local/bin/helm \
    && curl -fsSL -o /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl" \
    && chmod +x /usr/local/bin/kubectl \
    && curl -fsSL "https://github.com/golang-migrate/migrate/releases/download/${MIGRATE_VERSION}/migrate.linux-${ARCH}.tar.gz" \
       | tar -xz -C /usr/local/bin migrate \
    && chmod +x /usr/local/bin/migrate \
    && rm -rf "/tmp/linux-${ARCH}" \
    && apk del curl
COPY --from=builder /bin/api /usr/local/bin/api
COPY configs/ /etc/nullus/configs/
COPY db/migrations/ /etc/nullus/migrations/
EXPOSE 8080
CMD ["api"]
