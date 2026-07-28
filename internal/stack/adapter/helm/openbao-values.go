package helm

import (
	"context"
	"fmt"
	"strings"
)

const (
	// OpenBaoImageRepository / OpenBaoImageTag 는 OpenBao 이미지를 고정한다.
	// 에어갭 번들은 재현 가능해야 하므로 latest 같은 mutable 태그를 쓰지 않는다.
	OpenBaoImageRepository = "openbao/openbao"
	OpenBaoImageTag        = "2.5.5"

	// OpenBaoKubectlImage 는 init Job 이 K8s Secret 을 만들 때 사용하는 이미지다.
	// OpenBao 이미지에는 kubectl 이 없어 별도 컨테이너가 필요하다.
	OpenBaoKubectlImage = "docker.io/bitnamilegacy/kubectl:1.33.4"

	// OpenBaoUnsealKeysSecret 은 init Job 이 만든 unseal key 와 root token 을 담는다.
	// 이 Secret 과 OpenBao PVC 는 생애주기를 함께한다 — 한쪽만 남으면 재설치 시
	// 금고를 열 수 없게 되므로 삭제도 항상 함께 수행한다.
	OpenBaoUnsealKeysSecret = "openbao-unseal-keys"

	// OpenBaoDataStorageSize 는 file 스토리지 백엔드용 PVC 크기다.
	OpenBaoDataStorageSize = "5Gi"

	openBaoUnsealSidecarName = "unseal"
	openBaoKeysMountPath     = "/openbao/unseal" // #nosec G101 -- 마운트 경로이며 자격증명이 아님
)

// unsealSidecarScript 는 로컬 seal-status 를 폴링하다가 봉인 상태면 key share 를 제출한다.
//
// OpenBao 파드 안에서 실행되므로 RBAC 이 전혀 필요 없다 — API 호출이 localhost 를
// 벗어나지 않기 때문이다. 이미 개방된 금고는 건드리지 않아 멱등하며, 차트 설치와
// init Job 완료 사이에 키 Secret 이 아직 없는 정상 상태도 견딘다.
const unsealSidecarScript = `set -eu
KEYS_DIR="` + openBaoKeysMountPath + `"
ADDR="http://127.0.0.1:8200"
while true; do
  status="$(wget -q -O - "${ADDR}/v1/sys/seal-status" 2>/dev/null || true)"
  case "${status}" in
    *'"sealed":true'*)
      if [ ! -d "${KEYS_DIR}" ]; then
        echo "unseal: 키가 아직 프로비저닝되지 않음, 대기"
      else
        for f in "${KEYS_DIR}"/key*; do
          [ -f "${f}" ] || continue
          key="$(cat "${f}")"
          [ -n "${key}" ] || continue
          wget -q -O /dev/null --header='Content-Type: application/json' \
            --post-data="{\"key\":\"${key}\"}" "${ADDR}/v1/sys/unseal" 2>/dev/null || true
        done
        echo "unseal: key share 제출 완료"
      fi
      ;;
    *'"sealed":false'*)
      : # 이미 개방됨 - 아무것도 하지 않는다
      ;;
    *)
      echo "unseal: openbao 응답 대기 중"
      ;;
  esac
  sleep 10
done
`

// openBaoStandaloneConfig 는 서버 HCL 설정을 생성한다.
// 단일 replica 구성이므로 raft 대신 file 스토리지를 쓰고,
// 컨테이너에는 보통 IPC_LOCK capability 가 없으므로 disable_mlock 을 켠다.
func openBaoStandaloneConfig() string {
	return `ui = true
disable_mlock = true

listener "tcp" {
  tls_disable     = 1
  address         = "[::]:8200"
  cluster_address = "[::]:8201"
}

storage "file" {
  path = "/openbao/data"
}
`
}

// openBaoValues 는 공식 OpenBao 차트에 넘길 Helm values 를 만든다.
//
// storageClass 는 설치 마법사에서 선택한 값이며, 비어 있으면 클러스터
// 기본 StorageClass 를 사용한다.
func openBaoValues(storageClass string) map[string]any {
	dataStorage := map[string]any{
		"enabled":      true,
		"size":         OpenBaoDataStorageSize,
		"mountPath":    "/openbao/data",
		"accessMode":   "ReadWriteOnce",
		"storageClass": nil,
	}
	if sc := strings.TrimSpace(storageClass); sc != "" {
		dataStorage["storageClass"] = sc
	}

	return map[string]any{
		"injector": map[string]any{
			"enabled": false,
		},
		"ui": map[string]any{
			"enabled": true,
		},
		"server": map[string]any{
			"image": map[string]any{
				"repository": OpenBaoImageRepository,
				"tag":        OpenBaoImageTag,
			},
			"dataStorage": dataStorage,
			"standalone": map[string]any{
				"enabled": true,
				"config":  openBaoStandaloneConfig(),
			},
			"ingress": map[string]any{
				"enabled": false,
			},
			"resources": map[string]any{
				"requests": map[string]any{
					"cpu":    "200m",
					"memory": "256Mi",
				},
				"limits": map[string]any{
					"cpu":    "500m",
					"memory": "512Mi",
				},
			},
			// unseal 사이드카는 키를 볼륨 마운트로만 받는다.
			// ServiceAccount 에 Secret read 권한을 주지 않는다.
			"volumes": []any{
				map[string]any{
					"name": "unseal-keys",
					"secret": map[string]any{
						"secretName": OpenBaoUnsealKeysSecret,
						"optional":   true,
					},
				},
			},
			"extraContainers": []any{
				map[string]any{
					"name":            openBaoUnsealSidecarName,
					"image":           fmt.Sprintf("%s:%s", OpenBaoImageRepository, OpenBaoImageTag),
					"imagePullPolicy": "IfNotPresent",
					"command":         []any{"/bin/sh", "-c", unsealSidecarScript},
					"volumeMounts": []any{
						map[string]any{
							"name":      "unseal-keys",
							"mountPath": openBaoKeysMountPath,
							"readOnly":  true,
						},
					},
					"resources": map[string]any{
						"requests": map[string]any{
							"cpu":    "10m",
							"memory": "32Mi",
						},
						"limits": map[string]any{
							"cpu":    "50m",
							"memory": "64Mi",
						},
					},
				},
			},
		},
	}
}

// stackStorageClass 는 이 스택에 선택된 StorageClass 를 돌려준다.
func (o *Orchestrator) stackStorageClass() string {
	o.mu.Lock()
	cfg := o.stackConfig
	o.mu.Unlock()
	if cfg == nil || cfg.Storage == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Storage.StorageClass)
}

// ensureStorageClassExists 는 선택된 StorageClass 가 대상 클러스터에 실제로
// 존재하는지 확인한다. 설정 저장 이후 SC 가 삭제·변경됐을 수 있으므로
// 설치 시작 시점에 다시 검증한다.
//
// 미선택(빈 값)이면 클러스터 기본 StorageClass 를 쓰겠다는 뜻이므로,
// 기본 SC 존재 여부를 확인한다. 둘 다 없으면 PVC 가 Pending 에 머물러
// 설치가 멈추므로 여기서 차단한다.
func (o *Orchestrator) ensureStorageClassExists(ctx context.Context) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}

	selected := o.stackStorageClass()
	if selected != "" {
		if _, err := o.runKubectl(ctx, "get", "storageclass", selected); err != nil {
			return fmt.Errorf("선택한 StorageClass %q 를 대상 클러스터에서 찾을 수 없습니다: %w", selected, err)
		}
		return nil
	}

	out, err := o.runKubectl(ctx, "get", "storageclass",
		"-o", `jsonpath={range .items[*]}{.metadata.annotations.storageclass\.kubernetes\.io/is-default-class}{"\n"}{end}`)
	if err != nil {
		// 조회 자체가 실패하면 권한/연결 문제이므로 설치를 막지 않고 후속 스텝에서 판단한다.
		return nil
	}
	if strings.Contains(string(out), "true") {
		return nil
	}
	return fmt.Errorf("대상 클러스터에 기본 StorageClass 가 없습니다. 설치 마법사에서 StorageClass 를 선택하세요")
}
