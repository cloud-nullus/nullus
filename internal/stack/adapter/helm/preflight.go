package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// PreflightNamespace 는 설치를 시작하기 전에 그 자리가 비어 있는지 본다.
//
// 이전 설치의 볼륨이 남아 있으면 새 설치가 옛 데이터베이스를 물려받는다. 그 안의
// 비밀번호는 이번에 새로 만든 Secret 과 다르고, 그 사실은 한참 뒤에 엉뚱한 도구의
// 오류로 드러난다 — PostgreSQL 은 Gitea 의 28P01 로, Harbor 는 프로비저닝 401 로
// 나왔다. 둘 다 20분을 태운 뒤였다.
//
// 여기서 멈추면 몇 초 만에 알 수 있다.
func (o *Orchestrator) PreflightNamespace(ctx context.Context, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	target := strings.TrimSpace(namespace)
	if target == "" {
		return nil
	}

	out, err := o.runKubectl(ctx, "get", "pvc", "-n", target, "-o", "json")
	if err != nil {
		// 네임스페이스가 아직 없으면 조회가 실패한다 — 새 설치라는 뜻이라 통과다.
		// 연결 자체가 안 되는 경우도 여기로 오지만, 그건 다음 단계가 곧 잡는다.
		return nil
	}

	found := classifyLeftovers(string(out))
	if len(found.Names) == 0 {
		return nil
	}
	// 복구가 놓아둔 볼륨은 막지 않는다 — 복구는 볼륨과 그 시점의 Secret·금고를
	// 함께 되돌리므로, 이 가드가 막으려는 비밀번호 어긋남이 생기지 않는다.
	if found.AllRestoredFromSameBackup {
		return nil
	}
	return leftoverVolumeError(target, found.Names)
}

// leftovers 는 네임스페이스에 남은 볼륨과 그 출처를 말한다.
type leftovers struct {
	Names []string
	// AllRestoredFromSameBackup 은 남은 볼륨이 **전부** 같은 복구에서 왔음을 뜻한다.
	// 하나라도 출처가 없거나 서로 다르면 거짓이다 — 그때는 어느 시점의 상태인지
	// 말할 수 없고, 말할 수 없으면 막는 편이 싸다.
	AllRestoredFromSameBackup bool
	BackupRunID               string
}

// classifyLeftovers 는 `kubectl get pvc -o json` 출력을 읽는다.
//
// 클러스터 없이 검증할 수 있도록 분리했다 — 이 판정이 틀리면 설치가 부당하게
// 막히거나(복구본을 잔여로 오인), 막아야 할 것을 통과시킨다(잔여를 복구본으로 오인).
// 뒤쪽이 특히 비싸다: 20분 뒤 엉뚱한 도구의 인증 오류로 드러난다.
func classifyLeftovers(pvcListJSON string) leftovers {
	var list struct {
		Items []struct {
			Metadata struct {
				Name        string            `json:"name"`
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(pvcListJSON), &list); err != nil {
		// 읽을 수 없으면 판단하지 않는다 — 이름을 못 모으므로 호출부는 통과시킨다.
		return leftovers{}
	}

	out := leftovers{Names: make([]string, 0, len(list.Items))}
	marks := make(map[string]bool)
	for _, item := range list.Items {
		out.Names = append(out.Names, item.Metadata.Name)
		marks[item.Metadata.Annotations[shareddomain.RestoredFromBackupAnnotation]] = true
	}
	if len(out.Names) == 0 {
		return out
	}
	// 표시가 하나뿐이고 그것이 비어 있지 않아야 "전부 같은 복구본" 이다.
	if len(marks) == 1 {
		for id := range marks {
			if id != "" {
				out.AllRestoredFromSameBackup = true
				out.BackupRunID = id
			}
		}
	}
	return out
}

// leftoverVolumeError 는 무엇이 남았고 왜 문제이며 어떻게 푸는지까지 적는다.
//
// "볼륨이 남았습니다" 만으로는 사용자가 무엇을 해야 하는지 알 수 없다. 이 메시지가
// 설치 실패 화면에 그대로 뜨므로, 여기서 끝까지 말해 주지 않으면 로그를 뒤지게 된다.
func leftoverVolumeError(namespace string, leftovers []string) error {
	return fmt.Errorf(
		"네임스페이스 %s 에 이전 설치의 볼륨이 남아 있습니다: %s. "+
			"이대로 설치하면 옛 데이터베이스를 물려받아 이번에 새로 만든 비밀번호와 어긋나고, "+
			"한참 뒤 Gitea 의 인증 실패나 Harbor 의 401 로 드러납니다. "+
			"네임스페이스를 통째로 지우거나(kubectl delete namespace %s) "+
			"볼륨을 지운 뒤(kubectl -n %s delete pvc %s) 다시 설치하세요",
		namespace, strings.Join(leftovers, ", "),
		namespace, namespace, strings.Join(leftovers, " "))
}
