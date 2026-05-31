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
RUN apk add --no-cache ca-certificates tzdata curl \
    && ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" \
    && curl -fsSL "https://get.helm.sh/helm-${HELM_VERSION}-linux-${ARCH}.tar.gz" | tar -xz -C /tmp \
    && mv "/tmp/linux-${ARCH}/helm" /usr/local/bin/helm \
    && curl -fsSL -o /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl" \
    && chmod +x /usr/local/bin/kubectl \
    && rm -rf "/tmp/linux-${ARCH}" \
    && apk del curl
COPY --from=builder /bin/api /usr/local/bin/api
COPY configs/ /etc/nullus/configs/
COPY db/migrations/ /etc/nullus/migrations/
EXPOSE 8080
CMD ["api"]
