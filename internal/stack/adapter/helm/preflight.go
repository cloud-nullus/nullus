package helm

import (
	"context"
	"fmt"
	"strings"
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

	out, err := o.runKubectl(ctx, "get", "pvc", "-n", target, "-o", "name")
	if err != nil {
		// 네임스페이스가 아직 없으면 조회가 실패한다 — 새 설치라는 뜻이라 통과다.
		// 연결 자체가 안 되는 경우도 여기로 오지만, 그건 다음 단계가 곧 잡는다.
		return nil
	}

	if leftovers := parsePVCNames(string(out)); len(leftovers) > 0 {
		return leftoverVolumeError(target, leftovers)
	}
	return nil
}

func parsePVCNames(output string) []string {
	names := make([]string, 0, 4)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if index := strings.LastIndex(trimmed, "/"); index >= 0 {
			trimmed = trimmed[index+1:]
		}
		names = append(names, trimmed)
	}
	return names
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
