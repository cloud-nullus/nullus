package helm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	openBaoBootstrapJobName = "openbao-bootstrap"

	// OpenBaoKVMount 는 KV v2 시크릿 엔진의 마운트 이름이다.
	//
	// 경로 규약(`kv/nullus/{env}/{org}/...`)과 실제 마운트를 일치시키기 위해
	// "secret" 이 아니라 "kv" 로 마운트한다. 운영 모드는 dev 모드와 달리
	// 엔진이 자동 마운트되지 않으므로 부트스트랩에서 명시적으로 활성화한다.
	OpenBaoKVMount = "kv"

	// OpenBao Kubernetes Auth role 이름
	OpenBaoControllerRole = "nullus-controller"
	OpenBaoESORole        = "nullus-eso"

	// role 에 바인딩되는 ServiceAccount 이름
	OpenBaoControllerServiceAccount = "nullus-controller"
	OpenBaoESOServiceAccount        = "external-secrets"

	openBaoBootstrapTimeout = 5 * time.Minute

	// 컨텍스트는 kubectl --timeout 보다 넉넉해야 한다. 같은 값이면 컨텍스트가
	// 먼저(또는 동시에) 만료돼 프로세스가 kill 되고, kubectl 이 남기는 실제
	// 원인 대신 "signal: killed" 만 보인다.
	openBaoBootstrapWaitContextTimeout = openBaoBootstrapTimeout + 30*time.Second
)

// openBaoBootstrapScript 는 시크릿 엔진·인증 백엔드·정책·role 을 구성한다.
//
// 모든 명령이 멱등하다 — 엔진/인증 활성화는 이미 존재하면 무시하고,
// policy/role write 는 덮어쓰기다. 따라서 스텝 재시도가 안전하다.
//
// auth/kubernetes/config 에 token_reviewer_jwt 와 kubernetes_ca_cert 를 넣지
// 않는 것이 핵심이다. 생략하면 OpenBao 가 자기 파드의 로컬 SA 토큰을 매 요청마다
// 새로 읽어 TokenReview 에 사용하므로,
//   - Kubernetes 1.24+ 에서 약 1년 뒤 만료되는 문제와
//   - 정적 토큰이 파드에 바인딩되어 재시작 시 403 이 되는 문제
//
// 를 모두 처음부터 피한다.
const openBaoBootstrapScript = `set -eu
export BAO_ADDR="http://openbao.${NAMESPACE}.svc.cluster.local:8200"

# root token 은 부트스트랩 성공 뒤 폐기되고 Secret 에서도 제거된다(설계대로).
# 따라서 두 번째 실행부터는 BAO_TOKEN 이 비어 있는 상태로 들어온다.
#
# 이때 부트스트랩 완료 여부를 인증이 필요한 호출(bao list 등)로 확인하면
# 확인 자체가 불가능해 항상 실패한다. Kubernetes Auth 로그인으로 확인한다 —
# 로그인에 성공한다는 것은 auth 활성화 + role + policy 가 모두 갖춰졌다는 뜻이다.
if [ -z "${BAO_TOKEN:-}" ] || ! bao token lookup >/dev/null 2>&1; then
  echo "root token 을 쓸 수 없음 — Kubernetes Auth 로 부트스트랩 여부 확인"
  SA_TOKEN_PATH=/var/run/secrets/kubernetes.io/serviceaccount/token
  if [ -r "${SA_TOKEN_PATH}" ] && bao write -field=token \
      auth/kubernetes/login \
      role=` + OpenBaoControllerRole + ` \
      jwt="$(cat ${SA_TOKEN_PATH})" >/dev/null 2>&1; then
    echo "부트스트랩 완료 상태로 확인됨 - 건너뜀"
    exit 0
  fi
  echo "root token 없이 부트스트랩할 수 없습니다 (Kubernetes Auth 로그인 실패)" >&2
  exit 1
fi

echo "== KV v2 엔진 활성화 =="
if bao secrets list -format=json | grep -q "\"${KV_MOUNT}/\""; then
  echo "${KV_MOUNT}/ 이미 마운트됨"
else
  bao secrets enable -path="${KV_MOUNT}" kv-v2
fi

echo "== Kubernetes Auth 활성화 =="
if bao auth list -format=json | grep -q '"kubernetes/"'; then
  echo "kubernetes/ 이미 활성화됨"
else
  bao auth enable kubernetes
fi

echo "== Kubernetes Auth 설정 (리뷰어 토큰 생략) =="
bao write auth/kubernetes/config kubernetes_host="https://kubernetes.default.svc"

echo "== 정책 작성 =="
bao policy write ` + OpenBaoControllerRole + `-write - <<POLICY
path "${KV_MOUNT}/data/nullus/*" {
  capabilities = ["create", "update", "read"]
}
path "${KV_MOUNT}/metadata/nullus/*" {
  capabilities = ["read", "list"]
}
POLICY

bao policy write ` + OpenBaoESORole + `-read - <<POLICY
path "${KV_MOUNT}/data/nullus/*" {
  capabilities = ["read"]
}
path "${KV_MOUNT}/metadata/nullus/*" {
  capabilities = ["read", "list"]
}
POLICY

echo "== Role 작성 =="
bao write auth/kubernetes/role/` + OpenBaoControllerRole + ` \
  bound_service_account_names="` + OpenBaoControllerServiceAccount + `" \
  bound_service_account_namespaces="${NAMESPACE}" \
  policies="` + OpenBaoControllerRole + `-write" \
  ttl=1h

bao write auth/kubernetes/role/` + OpenBaoESORole + ` \
  bound_service_account_names="` + OpenBaoESOServiceAccount + `" \
  bound_service_account_namespaces="${NAMESPACE}" \
  policies="` + OpenBaoESORole + `-read" \
  ttl=1h

echo "부트스트랩 완료"

# REVOKE_ROOT 가 켜져 있으면 마지막에 root token 을 폐기한다.
# unseal key 가 있으면 operator generate-root 로 재발급할 수 있으므로 안전하다.
if [ "${REVOKE_ROOT:-false}" = "true" ]; then
  echo "root token 폐기"
  bao token revoke -self
fi
`

// openBaoBootstrapManifest 는 부트스트랩 Job 과 컨트롤러용 ServiceAccount 를 만든다.
//
// 컨트롤러 SA 는 Nullus 백엔드가 TokenRequest 로 단기 토큰을 발급받아
// OpenBao 에 로그인할 때 사용하는 신원이다. 클러스터 권한은 필요 없다 —
// OpenBao 가 TokenReview 로 "이 SA 가 맞다" 는 것만 확인하면 되기 때문이다.
func openBaoBootstrapManifest(namespace string, revokeRoot bool) string {
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}
	baoImage := fmt.Sprintf("%s:%s", OpenBaoImageRepository, OpenBaoImageTag)

	return fmt.Sprintf(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: %[1]s
  namespace: %[2]s
---
apiVersion: batch/v1
kind: Job
metadata:
  name: %[3]s
  namespace: %[2]s
spec:
  backoffLimit: 3
  template:
    spec:
      restartPolicy: OnFailure
      # role 에 바인딩된 SA 로 떠야 Kubernetes Auth 로그인이 통과한다.
      # default SA 로 뜨면 재실행 시 부트스트랩 완료 확인이 거부된다.
      serviceAccountName: %[1]s
      containers:
        - name: bootstrap
          image: %[4]s
          imagePullPolicy: IfNotPresent
          env:
            - name: NAMESPACE
              value: %[2]s
            - name: KV_MOUNT
              value: %[5]s
            - name: REVOKE_ROOT
              value: "%[8]t"
            - name: BAO_TOKEN
              valueFrom:
                secretKeyRef:
                  name: %[6]s
                  key: root-token
                  optional: true
          command: ["/bin/sh", "-c"]
          args:
            - |
%[7]s
`, OpenBaoControllerServiceAccount, namespace, openBaoBootstrapJobName, baoImage,
		OpenBaoKVMount, OpenBaoUnsealKeysSecret,
		indentScript(openBaoBootstrapScript, "              "), revokeRoot)
}

// runOpenBaoBootstrap 은 부트스트랩 Job 을 적용하고 완료를 기다린다.
func (o *Orchestrator) runOpenBaoBootstrap(ctx context.Context, namespace string) error {
	return o.runOpenBaoBootstrapJob(ctx, namespace, false)
}

// runOpenBaoBootstrapJob 은 부트스트랩 Job 을 적용하고 완료를 기다린다.
// revokeRoot 가 true 면 작업 마지막에 root token 을 폐기한다.
func (o *Orchestrator) runOpenBaoBootstrapJob(ctx context.Context, namespace string, revokeRoot bool) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = o.namespace
	}

	if _, err := o.runKubectl(ctx, "delete", "job", openBaoBootstrapJobName,
		"-n", namespace, "--ignore-not-found"); err != nil {
		return fmt.Errorf("이전 openbao bootstrap job 정리 실패: %w", err)
	}

	if err := o.applyManifest(ctx, namespace, openBaoBootstrapManifest(namespace, revokeRoot)); err != nil {
		return fmt.Errorf("openbao bootstrap job 적용 실패: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, openBaoBootstrapWaitContextTimeout)
	defer cancel()
	if _, err := o.runKubectl(waitCtx, "wait", "--for=condition=complete",
		fmt.Sprintf("job/%s", openBaoBootstrapJobName), "-n", namespace,
		fmt.Sprintf("--timeout=%ds", int(openBaoBootstrapTimeout.Seconds()))); err != nil {
		// 실패 원인은 job 파드 로그에만 있다. 대기 오류만 올리면
		// "timed out waiting for the condition" 밖에 남지 않아 진단이 불가능하다.
		logs, logErr := o.runKubectl(ctx, "logs", fmt.Sprintf("job/%s", openBaoBootstrapJobName),
			"-n", namespace, "--tail=40")
		if logErr != nil || strings.TrimSpace(string(logs)) == "" {
			return fmt.Errorf("openbao bootstrap job 완료 대기 실패: %w", err)
		}
		return fmt.Errorf("openbao bootstrap job 완료 대기 실패: %w (job logs: %s)",
			err, strings.TrimSpace(string(logs)))
	}
	return nil
}

// RevokeOpenBaoRootToken 은 root token 을 폐기하고 Secret 에서도 제거한다.
//
// Kubernetes Auth 가 실제로 동작한다는 것이 확인된 뒤에 호출해야 한다.
// 그 전에 폐기하면 인증 경로가 하나도 남지 않는다.
//
// 폐기해도 복구할 수 있다 — unseal key threshold 를 충족하면
// `bao operator generate-root` 로 재발급할 수 있고, 키는 같은 클러스터의
// openbao-unseal-keys Secret 에 있다.
func (o *Orchestrator) RevokeOpenBaoRootToken(ctx context.Context, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = o.namespace
	}

	// 이미 제거되었으면 아무것도 하지 않는다 (멱등).
	out, err := o.runKubectl(ctx, "get", "secret", OpenBaoUnsealKeysSecret, "-n", namespace,
		"-o", "jsonpath={.data.root-token}")
	if err != nil || strings.TrimSpace(string(out)) == "" {
		slog.Info("root token 이 이미 폐기된 상태입니다", "namespace", namespace)
		return nil
	}

	// 부트스트랩을 재적용하며 마지막 단계에서 self revoke 한다.
	if err := o.runOpenBaoBootstrapJob(ctx, namespace, true); err != nil {
		return fmt.Errorf("root token 폐기 실패: %w", err)
	}

	// Secret 에서도 값을 지운다. 남겨두면 폐기된 토큰이 유효한 것처럼 보인다.
	if _, err := o.runKubectl(ctx, "patch", "secret", OpenBaoUnsealKeysSecret, "-n", namespace,
		"--type=json", "-p", `[{"op":"remove","path":"/data/root-token"}]`); err != nil {
		return fmt.Errorf("root token 키 제거 실패: %w", err)
	}
	slog.Info("root token 을 폐기하고 Secret 에서 제거했습니다", "namespace", namespace)
	return nil
}
