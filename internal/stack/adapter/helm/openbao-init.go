package helm

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	openBaoInitJobName        = "openbao-init"
	openBaoInitServiceAccount = "openbao-init"

	// OpenBaoKeyShares / OpenBaoKeyThreshold 기본값은 1/1 이다.
	// auto-unseal 구성에서는 threshold 를 채우는 데 필요한 모든 조각이
	// 어차피 같은 Secret 안에 있으므로, 분할이 런타임 보안을 높이지 않고
	// 복잡도만 늘린다. 오프라인 백업본을 여러 관리자에게 나누려는 조직만
	// 이 값을 올리면 된다.
	OpenBaoKeyShares    = 1
	OpenBaoKeyThreshold = 1

	openBaoInitJobTimeout = 5 * time.Minute
)

// openBaoInitProbeScript 는 initContainer 에서 실행되며 실제 초기화를 수행한다.
//
// 멱등성 가드가 이 스크립트의 핵심이다. 스택 설치에는 스텝 재시도 경로가 있어서,
// 이미 초기화된 금고에 operator init 을 다시 돌리면 기존 unseal key 가 무효화되고
// 금고가 영구히 봉인된다. 그래서 /v1/sys/init 을 먼저 확인하고 분기한다.
//
// 초기화 결과는 파드 내부 emptyDir 로만 전달되므로 unseal key 가 대상 클러스터를
// 벗어나지 않는다.
const openBaoInitProbeScript = `set -eu
ADDR="http://openbao.${NAMESPACE}.svc.cluster.local:8200"
export BAO_ADDR="${ADDR}"

echo "openbao 응답 대기"
i=0
while [ $i -lt 60 ]; do
  if wget -q -O - "${ADDR}/v1/sys/seal-status" >/dev/null 2>&1; then break; fi
  i=$((i+1))
  sleep 5
done

INIT_STATUS="$(wget -q -O - "${ADDR}/v1/sys/init" 2>/dev/null || echo '')"
echo "init 상태: ${INIT_STATUS}"

case "${INIT_STATUS}" in
  *'"initialized":true'*)
    echo "이미 초기화됨 - init 건너뜀 (멱등)"
    touch /shared/skip
    exit 0
    ;;
esac

echo "초기화 수행 (shares=${KEY_SHARES} threshold=${KEY_THRESHOLD})"
bao operator init -key-shares="${KEY_SHARES}" -key-threshold="${KEY_THRESHOLD}" > /shared/init.txt
echo "초기화 완료"
`

// openBaoInitSecretScript 는 초기화 결과를 K8s Secret 으로 만든다.
// kubectl 이 필요해 OpenBao 이미지가 아닌 별도 컨테이너에서 실행된다.
const openBaoInitSecretScript = `set -eu
if [ -f /shared/skip ]; then
  echo "초기화를 건너뛰었으므로 Secret 생성도 생략"
  exit 0
fi
if [ ! -s /shared/init.txt ]; then
  echo "init 출력이 비어 있음" >&2
  exit 1
fi

set --
i=1
while :; do
  key="$(sed -n "s/^Unseal Key ${i}: //p" /shared/init.txt)"
  [ -n "${key}" ] || break
  set -- "$@" "--from-literal=key${i}=${key}"
  i=$((i+1))
done
if [ $# -eq 0 ]; then
  echo "unseal key 를 찾지 못함" >&2
  exit 1
fi

root="$(sed -n 's/^Initial Root Token: //p' /shared/init.txt)"
if [ -z "${root}" ]; then
  echo "root token 을 찾지 못함" >&2
  exit 1
fi
set -- "$@" "--from-literal=root-token=${root}"

kubectl create secret generic "${SECRET_NAME}" -n "${NAMESPACE}" "$@" \
  --dry-run=client -o yaml \
  | kubectl apply -f -
echo "unseal key Secret 생성 완료"
`

// openBaoInitManifest 는 init Job 과 필요한 RBAC 을 렌더링한다.
//
// Role 은 해당 네임스페이스의 Secret 생성/갱신으로만 제한한다.
func openBaoInitManifest(namespace string) string {
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
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: %[1]s
  namespace: %[2]s
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["create", "get", "patch", "update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: %[1]s
  namespace: %[2]s
subjects:
  - kind: ServiceAccount
    name: %[1]s
    namespace: %[2]s
roleRef:
  kind: Role
  name: %[1]s
  apiGroup: rbac.authorization.k8s.io
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
      serviceAccountName: %[1]s
      volumes:
        - name: shared
          emptyDir:
            medium: Memory
      initContainers:
        - name: init
          image: %[4]s
          imagePullPolicy: IfNotPresent
          env:
            - name: NAMESPACE
              value: %[2]s
            - name: KEY_SHARES
              value: "%[5]d"
            - name: KEY_THRESHOLD
              value: "%[6]d"
          volumeMounts:
            - name: shared
              mountPath: /shared
          command: ["/bin/sh", "-c"]
          args:
            - |
%[7]s
      containers:
        - name: store
          image: %[8]s
          imagePullPolicy: IfNotPresent
          env:
            - name: NAMESPACE
              value: %[2]s
            - name: SECRET_NAME
              value: %[9]s
          volumeMounts:
            - name: shared
              mountPath: /shared
          command: ["/bin/sh", "-c"]
          args:
            - |
%[10]s
`, openBaoInitServiceAccount, namespace, openBaoInitJobName, baoImage,
		OpenBaoKeyShares, OpenBaoKeyThreshold, indentScript(openBaoInitProbeScript, "              "),
		OpenBaoKubectlImage, OpenBaoUnsealKeysSecret, indentScript(openBaoInitSecretScript, "              "))
}

// indentScript 는 YAML 블록 스칼라 안에 넣을 수 있도록 스크립트 전체를 들여쓴다.
func indentScript(script, indent string) string {
	lines := strings.Split(strings.TrimRight(script, "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

// runOpenBaoInit 은 init Job 을 적용하고 완료를 기다린다.
func (o *Orchestrator) runOpenBaoInit(ctx context.Context, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = o.namespace
	}

	// Job 은 immutable 이므로 이전 설치가 남긴 Job 을 먼저 제거해야 재적용할 수 있다.
	// 금고 초기화 여부는 Job 존재가 아니라 스크립트의 /v1/sys/init 확인으로 판단하므로
	// 여기서 Job 을 지워도 재초기화가 일어나지 않는다.
	if _, err := o.runKubectl(ctx, "delete", "job", openBaoInitJobName,
		"-n", namespace, "--ignore-not-found"); err != nil {
		return fmt.Errorf("이전 openbao init job 정리 실패: %w", err)
	}

	if err := o.applyManifest(ctx, namespace, openBaoInitManifest(namespace)); err != nil {
		return fmt.Errorf("openbao init job 적용 실패: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, openBaoInitJobTimeout)
	defer cancel()
	if _, err := o.runKubectl(waitCtx, "wait", "--for=condition=complete",
		fmt.Sprintf("job/%s", openBaoInitJobName), "-n", namespace,
		fmt.Sprintf("--timeout=%ds", int(openBaoInitJobTimeout.Seconds()))); err != nil {
		return fmt.Errorf("openbao init job 완료 대기 실패: %w", err)
	}

	return o.waitForOpenBaoUnsealed(ctx, namespace)
}

// waitForOpenBaoUnsealed 는 사이드카가 금고를 열 때까지 기다린다.
//
// preflight gate 의 차단 조건이다. 봉인이 풀리지 않은 상태로 후속 스텝을 진행하면
// 시크릿을 쓰지도 읽지도 못하므로, 여기서 실패하면 설치를 중단시킨다.
func (o *Orchestrator) waitForOpenBaoUnsealed(ctx context.Context, namespace string) error {
	const (
		maxAttempts = 30
		retryDelay  = 5 * time.Second
	)

	var lastStatus string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		out, err := o.runKubectl(ctx, "exec", "-n", namespace, "openbao-0", "-c", "openbao",
			"--", "wget", "-q", "-O", "-", "http://127.0.0.1:8200/v1/sys/seal-status")
		if err == nil {
			lastStatus = strings.TrimSpace(string(out))
			if strings.Contains(lastStatus, `"sealed":false`) {
				return nil
			}
		}
		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}
	return fmt.Errorf("openbao 봉인이 해제되지 않았습니다 (마지막 상태: %s)", lastStatus)
}
